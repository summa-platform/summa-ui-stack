#!/usr/bin/env python3

import sys, socket, json, traceback
import asyncio
from datetime import datetime
from collections import defaultdict
import hashlib
# import ssl

import asyncpg, yaml
# import aiohttp, certifi


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

    config.retry_timeout = 10 if config.retry_timeout is None else float(config.retry_timeout)
    config.retry_count = 20 if config.retry_count is None else int(config.retry_count)

    return config


async def add_missing_news_items_topics_column_data(pool):
    async with pool.acquire() as conn:
        print('Updating news items topics oolumn...')
        async with conn.transaction():
            i = 0
            async for r in conn.cursor("""
                    SELECT id, topics, data, jsonb_array_length("data"->'detectedTopics') FROM news_items
                    WHERE jsonb_array_length(data->'detectedTopics') > 0 AND array_length(topics,1) IS NULL;
                """,
                ):
                mediaItem = json.loads(r['data'], object_hook=Dict)
                mediaItem.detectedTopics.sort(key=lambda item: item[1], reverse=True)

                await conn.execute("""
                    UPDATE news_items SET topics = $2, topic_weights = $3 WHERE id = $1;
                    """,
                    mediaItem.id,
                    [topic[0] for topic in mediaItem.detectedTopics],
                    [float(topic[1]) if type(topic[1]) is str else topic[1] for topic in mediaItem.detectedTopics],
                )

                i += 1
                if i % 1000 == 0:
                    print('%ik' % (int(i/1000)))
            print('%i rows updated' % i)


async def convert_user_passwords(pool):
    async with pool.acquire() as conn:
        print('Updating users...')
        async with conn.transaction():
            i = 0
            async for r in conn.cursor("""
                SELECT id, password FROM users WHERE length(password) != 32;
                """,
                ):
                id, password = r['id'], r['password']
                # convert password to MD5 sum
                h = hashlib.md5()
                h.update(password.encode('utf8'))
                password = h.hexdigest()
                await conn.execute("""
                    UPDATE users SET password = $2 WHERE id = $1;
                    """,
                    id,
                    password,
                )
                i += 1
            print('%i rows updated' % i)


async def main(args, loop=None):
    print('Connecting to database', file=sys.stderr)
    while True:
        try:
            pool = await asyncpg.create_pool(
                    user=config.db.user,
                    password=config.db.password,
                    port=config.db.port or 5432,
                    host=config.db.host or 'localhost',
                    database=config.db.dbname,
                    server_settings=dict(application_name=config.db.app_name or 'summa-ui-updatedb'),
                    min_size=1,
                    max_size=config.db.pool_max_connections or 3,
            )
            break
        except (ConnectionRefusedError, asyncpg.exceptions.CannotConnectNowError, socket.gaierror):
            print('Error connecting to database, will reconnect', file=sys.stderr)
            await asyncio.sleep(config.db.reconnect_sleep or 10)

    await convert_user_passwords(pool)
    await add_missing_news_items_topics_column_data(pool)



if __name__ == "__main__":

    import argparse

    parser = argparse.ArgumentParser(description='UI Stack DB Migration Update Service', formatter_class=argparse.ArgumentDefaultsHelpFormatter)
    parser.add_argument('--config', '-c', metavar='FILE', type=str, default='config.yaml', help='database configuration file')
    parser.add_argument('--verbose', '-v', action='store_true', help='verbose mode')

    args = parser.parse_args()

    if not args.verbose:
        verb = lambda *args, **kwargs: None

    config = load_config(args.config or 'config.yaml')

    try:
        loop = asyncio.get_event_loop()
        loop.run_until_complete(main(args))
    except KeyboardInterrupt:
        print('INTERRUPTED')
