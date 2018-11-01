#!/usr/bin/env python3

import sys, ssl, socket, traceback, json
import asyncio
from datetime import datetime
from collections import defaultdict
from urllib.parse import quote

import asyncpg, aiohttp, certifi, yaml, async_timeout


class InternalServiceError(Exception): pass


class Dict(dict):
    def __init__(self, *args, **kwargs):
        super(Dict, self).__init__(*args, **kwargs)
        self.__dict__ = self
    def __getattribute__(self, key):
        try: return super(Dict, self).__getattribute__(key)
        except: return
    def __delattr__(self, name):
        if name in self: del self[name]

def json_Dict(s, *args, **kwargs):
    return json.loads(s, *args, object_hook=Dict, **kwargs)

def verb(*args, **kwargs):
    print(*args, file=sys.stderr, **kwargs)

def load_config(filename='config.yaml'):

    # https://www.programcreek.com/python/example/11269/yaml.add_constructor
    # https://pyyaml.org/wiki/PyYAMLDocumentation
    # https://stackoverflow.com/questions/36629559/how-to-use-custom-dictionary-class-while-loading-yaml

    def dict_constructor(loader, node):
        return Dict(loader.construct_mapping(node))

    yaml.add_constructor(yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, dict_constructor)

    with open(filename, 'r') as f:
        config = yaml.load(f)

    # normalization
    if config.clustering_api and config.clustering_api.endswith('/'):
        config.clustering_api = config.clustering_api.rstrip('/')
    # if config.main_api and config.main_api.endswith('/'):
    #     config.main_api = config.main_api.rstrip('/')
    if config.summarization_api and config.summarization_api.endswith('/'):
        config.summarization_api = config.summarization_api.rstrip('/')

    config.retry_timeout = 10 if config.retry_timeout is None else float(config.retry_timeout)
    config.retry_count = 20 if config.retry_count is None else int(config.retry_count)

    return config



async def get_geolocation_info(session, name):

    verb('Getting geolocation information for entity "%s"' % name)

    for i in range(5):
        try:
            if 'http' in name.lower():
                response = None
            else:
                # async with session.get(config.twofishes_api, params=dict(query=name)) as response:
                async with session.get(config.twofishes_api+quote(name)) as response:
                    if response.status != 200:
                        raise Exception('twofishes response %s: %s' % (str(response.status), await response.text()))
                    response = json.loads(await response.text(), object_hook=Dict)

            if not response or not response.interpretations or len(response.interpretations) == 0:
                verb(' => empty')
                return

            lat = response.interpretations[0].feature.geometry.center.lat
            lng = response.interpretations[0].feature.geometry.center.lng
            verb(' => lat: %s   lng: %s' % (lat, lng))
            return lng, lat

        except aiohttp.client_exceptions.ServerDisconnectedError as e:
            if not e.message:
                print('twofishes disconnected, assuming bad input (entity "%s"), skipping' % name, file=sys.stderr)
            else:
                print('twofishes disconnected error (entity "%s"):' % name, e, file=sys.stderr)
            return
        except aiohttp.client_exceptions.ClientConnectorError as e:
            print('Error connecting to twofishes, wait 10 seconds and retry...', file=sys.stderr)
            await asyncio.sleep(10)

    raise InternalServiceError("service not available, timed out")


def set_config(cfg):
    global config
    config = cfg


async def main(config, loop=None):
    set_config(config)
    print('Connecting to database', file=sys.stderr)
    while True:
        try:
            pool = await asyncpg.create_pool(
                    user=config.db.user,
                    password=config.db.password,
                    port=config.db.port or 5432,
                    host=config.db.host or 'localhost',
                    database=config.db.dbname,
                    server_settings=dict(application_name=config.db.app_name or 'summa-ui-clustering'),
                    min_size=1,
                    max_size=config.db.pool_max_connections or 3,
            )
            break
        except (OSError, ConnectionRefusedError, asyncpg.exceptions.CannotConnectNowError, socket.gaierror):
            print('Error connecting to database, will reconnect', file=sys.stderr)
            await asyncio.sleep(config.db.reconnect_sleep or 10)

    sslcontext = ssl.create_default_context(cafile=certifi.where())
    async with aiohttp.ClientSession(connector=aiohttp.TCPConnector(ssl_context=sslcontext), read_timeout=20*60, conn_timeout=30) as session:

        async with pool.acquire() as roconn, pool.acquire() as rwconn:
            async with roconn.transaction():

                print('Retrieving geolocation information for entities...')

                while True:
                    try:
                        i = 0
                        async for row in roconn.cursor('SELECT id, baseform FROM entities WHERE geo IS NULL;'):
                            id = row['id']
                            baseform = row['baseform']

                            location = await get_geolocation_info(session, baseform)

                            if location:
                                lng, lat = location
                                await rwconn.execute("""
                                    UPDATE entities SET geo = (SELECT ST_SetSRID(ST_MakePoint($2, $3), 4326)) WHERE id = $1;
                                    """,
                                    id,
                                    lng,
                                    lat,
                                )
                            else:
                                await rwconn.execute("""
                                    UPDATE entities SET geo = (ST_GeographyFromText('POINT EMPTY')) WHERE id = $1;
                                    """,
                                    id,
                                )

                            i += 1
                            if i % 1000 == 0:
                                print('%ik' % (int(i/1000)))

                        print('%i entities updated' % i)
                        print('geolocation for all entities is retrieved')

                        print('Wait 5 minutes for new entities to be updated')
                        await asyncio.sleep(5*60, loop=loop)

                    except KeyboardInterrupt:
                        raise KeyboardInterrupt
                    except Exception as e:
                        traceback.print_exc()
                        raise InternalServiceError(str(e))
                        # print('will wait %s seconds and retry' % str(config.retry_timeout))
                        # await asyncio.sleep(float(config.retry_timeout), loop=loop)


async def run(config='config.yaml', verbose=False, loop=None):
    global verb
    if not verbose:
        verb = lambda *args, **kwargs: None
    if type(config) is str:
        config = load_config(config or 'config.yaml')
    await main(config, loop=loop)



if __name__ == "__main__":

    import argparse

    parser = argparse.ArgumentParser(description='UI Stack Geolocation Retrieving Service', formatter_class=argparse.ArgumentDefaultsHelpFormatter)
    parser.add_argument('--config', '-c', metavar='FILE', type=str, default='config.yaml', help='database configuration file')
    parser.add_argument('--verbose', '-v', action='store_true', help='verbose mode')

    args = parser.parse_args()

    if not args.verbose:
        verb = lambda *args, **kwargs: None

    config = load_config(args.config or 'config.yaml')

    try:
        loop = asyncio.get_event_loop()
        loop.run_until_complete(main(config))
    except KeyboardInterrupt:
        print('INTERRUPTED')
