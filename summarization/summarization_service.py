#!/usr/bin/env python3

import sys, ssl, socket, traceback, json
import asyncio
from datetime import datetime
from collections import defaultdict

import asyncpg, aiohttp, certifi, yaml, async_timeout


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



async def prepare_summarize_request_body_with_items_of_cluster(pool, clusterid):
    async with pool.acquire() as conn:
        rows = await conn.fetch("""
            SELECT id, last_changed,
                data#>'{title}'->>'english' title,
                data#>'{transcript,english}'->>'text' transcript,
                data#>'{mainText}'->>'english' maintext,
                data#>'{summary}' summary
            FROM news_items WHERE cluster_id = $1;
            --ORDER BY last_changed DESC LIMIT $2;
        """, clusterid)

        documents = []
        last_changed = None
        for row in rows:
            if not last_changed or last_changed < row['last_changed']:
                last_changed = row['last_changed']
            summary = json.loads(row['summary'])
            documents.append(dict(
                id = row['id'],
                instances = [dict(
                    title = row['title'],
                    body = row['maintext'] or row['transcript'],
                    metadata = dict(
                        language = 'en',    # https://en.wikipedia.org/wiki/List_of_ISO_639-1_codes
                        # optional
                        summary = summary if type(summary) is str else ' '.join(summary),
                        # date = '',
                        # originalLanguage = '',
                        # sourceChannel = '',
                        # tags = [],
                        # tokenizedText = [],
                        # topics = [],
                    ),
                )],
                # source = {},
                # instanceAlignments = [],
            ))

        if not documents:
            return

        return len(documents), last_changed, dict(
            documents = documents,
            # socialMediaDocuments = [],
            # metadata = dict(id = '', topic = '' ),
        )


async def get_clusters_to_summarize(pool):
    async with pool.acquire() as conn:
        rows = await conn.fetch("""
            SELECT cluster_id, last_changed, count, highlights_datetime, highlights_source_count
            FROM (
                SELECT cluster_id, max(last_changed) last_changed, count(news_items.id) count,
                    clusters.highlights_datetime highlights_datetime,
                    clusters.highlights_source_count highlights_source_count,
                    jsonb_array_length(clusters.highlights) highlights_count
                FROM news_items
                FULL JOIN clusters ON cluster_id = clusters.id
                WHERE cluster_id IS NOT NULL
                GROUP BY cluster_id, highlights_datetime, highlights_source_count, highlights_count
            ) t
            WHERE highlights_datetime IS NULL OR (
                highlights_datetime < last_changed
                AND (count-highlights_source_count > 0.1*highlights_source_count OR highlights_count = 0)
            );
        """)

        return [row['cluster_id'] for row in rows]


async def update_cluster_highlights(pool, clusterid, highlights, last_changed, count, title):
    async with pool.acquire() as conn:
        await conn.execute("""
            INSERT INTO clusters (id, highlights, highlights_datetime, highlights_source_count, title)
            VALUES($1, $2, $3, $4, $5)
            ON CONFLICT ( id ) DO UPDATE
            SET highlights = $2, highlights_datetime = $3, highlights_source_count = $4, title = $5;
            """,
            clusterid,
            json.dumps(highlights),
            last_changed,
            count,
            title,
        )


async def summarize_cluster(pool, session, clusterid):

    print('Summarizing cluster %i' % clusterid)
    r = await prepare_summarize_request_body_with_items_of_cluster(pool, clusterid)
    if not r:
        return
    count, last_changed, body = r

    while True:
        try:
            async with session.post(config.summarization_api, json=body, params=dict(model='media', maxWordCount=200)) as response:
                if response.status != 200:
                    raise Exception('summarization request response %s: %s' % (str(response.status), await response.text()))
                response = await response.json()
            highlights = []
            title = None
            for highlight in response['highlights']:
                text = highlight.get('highlight', '')
                if text and (not title or len(title) > len(text)):
                    title = text
                sources = highlight.get('sourceDocuments', [])
                for source in sources:
                    source['lng'] = source.get('language', '')
                    del source['language']
                highlights.append(dict(
                    txt = text,
                    lng = highlight.get('language', ''),
                    src = sources,
                ))
            await update_cluster_highlights(pool, clusterid, highlights, last_changed, count, title)
            break
        except aiohttp.client_exceptions.ClientConnectorError as e:
            print('Error connecting to summarization server, wait 10 seconds and retry...')
            await asyncio.sleep(10)


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

        print('Summarizing clusters...')
        while True:
            try:
                print('Retrieving clusters to summarize...')
                cluster_ids = await get_clusters_to_summarize(pool)
                print('%i clusters to summarize' % len(cluster_ids))
                for cluster_id in cluster_ids:
                    if cluster_id:
                        await summarize_cluster(pool, session, cluster_id)
                print('Wait 5 minutes for more clusters to be summarized')
                await asyncio.sleep(5*60, loop=loop)
            except KeyboardInterrupt:
                raise KeyboardInterrupt
            except Exception as e:
                traceback.print_exc()
                # print('error:', e, file=sys.stderr)
                print('will wait %s seconds and retry' % str(config.retry_timeout))
                await asyncio.sleep(float(config.retry_timeout), loop=loop)


async def run(config='config.yaml', verbose=False, loop=None):
    global verb
    if not verbose:
        verb = lambda *args, **kwargs: None
    if type(config) is str:
        config = load_config(config or 'config.yaml')
    await main(config, loop=loop)



if __name__ == "__main__":

    import argparse

    parser = argparse.ArgumentParser(description='UI Stack Cluster Summarization Service', formatter_class=argparse.ArgumentDefaultsHelpFormatter)
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
