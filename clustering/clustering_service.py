#!/usr/bin/env python3

import sys, ssl, socket, traceback, json
import asyncio
from datetime import datetime
from collections import defaultdict

import asyncpg, yaml

from clustering import Clustering


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
    # if config.clustering_api and config.clustering_api.endswith('/'):
    #     config.clustering_api = config.clustering_api.rstrip('/')
    # if config.main_api and config.main_api.endswith('/'):
    #     config.main_api = config.main_api.rstrip('/')

    config.retry_timeout = 10 if config.retry_timeout is None else float(config.retry_timeout)
    config.retry_count = 20 if config.retry_count is None else int(config.retry_count)

    return config



async def get_unclustered_documents(pool, count=100, exclude=set(), exclude2=set(), buckets=False, starting=None, starting_after=None, loop=None):

    def prepare(id, data, last_changed):
        document = json_Dict(data)
        document.id = id
        document.timeLastChanged = last_changed # datetime object
        return document

    for i in reversed(range(config.retry_count)):
        try:
            async with pool.acquire() as conn:
                conditions = []
                args = []
                if buckets:
                    conditions.append('cluster_id IS NOT NULL AND cluster_bucket_id IS NULL')
                else:
                    conditions.append('cluster_id IS NULL')
                if starting:
                    args.append(starting)
                    conditions.append('last_changed >= $%i' % len(args))
                if starting_after:
                    args.append(starting_after)
                    conditions.append('last_changed > $%i' % len(args))
                rows = await conn.fetch("""
                    SELECT id, data, last_changed FROM news_items WHERE {conditions} ORDER BY last_changed ASC{limit};
                """.format(limit=' LIMIT %i' % count if count > 0 else '', conditions=' AND '.join(conditions)), *args)

                return [prepare(row['id'], row['data'], row['last_changed']) for row in rows if row['id'] not in exclude and row['id'] not in exclude2]

        except (ConnectionRefusedError, socket.gaierror) as e:
            print('db error:', e)
            if i > 0:
                print('will wait %s seconds and retry' % str(config.retry_timeout))
                await asyncio.sleep(float(config.retry_timeout), loop=loop)

    return []


async def update_news_items(pool, clustered_documents, merges):

    async with pool.acquire() as conn:

        # document clusters
        for document in clustered_documents:

            verb(document['id'], '->', document['cluster']) #, '::', document.get('topics'))

            # update db
            await conn.execute("""
                UPDATE news_items SET cluster_id = $2 WHERE id = $1;
                """,
                document['id'],
                document['cluster'],
            )

        # merge clusters
        for new_cluster, old_clusters in merges.items():

            if not old_clusters:
                continue

            verb(new_cluster, '<-', ', '.join(map(str, sorted(old_clusters))))

            # update db
            await conn.execute("""
                UPDATE news_items SET cluster_id = $2 WHERE cluster_id = ANY($1);
                """,
                list(old_clusters),
                new_cluster,
            )


async def cluster_documents(pool, documents, loop=None):

    source_timestamp_format = '%Y-%m-%dT%H:%M:%S.%fZ'
    timestamp_format = '%Y-%m-%d %H:%M:%S.%f'

    merges = defaultdict(set)
    clustered_documents = []

    for mediaItem in documents:
        document = {
            'id': mediaItem.id,
            'text': ' '.join([
                # mediaItem.title.english or mediaItem.title.original or '',
                mediaItem.teaser.english or '',
                mediaItem.mainText.english or '',
                mediaItem.transcript.english.text or ''
            ]),
            # 'timestamp_format': timestamp_format,
            'timestamp': datetime.strptime(mediaItem.timeAdded, source_timestamp_format).strftime(timestamp_format),
            # 'language': 'en',
            # 'group_id': 'English',
            # 'media_item_type': mediaItem.mediaItemType,   # sourceItemType in newsItems API
            # 'source_feed_name': mediaItem.source.id       # feedURL in newsItems API
        }
        response = clustering.add(document)
        # print(mediaItem.id, response)
        if response.merged:
            merges[response.cluster] |= set(response.merged)
        document['cluster'] = response.cluster
        clustered_documents.append(document)
    
    # 50 <- 63, 71
    # 48 <- 50, 62, 63
    # 49 <- 48, 62, 63
    # 47 <- 49, 58, 62, 63
    # print(merges)
    # for i in range(len(merges)):
    #     target_clusters = set(merges.keys())
    #     for new_cluster, old_clusters in list(merges.items()):
    #         for target_cluster in list(target_clusters):
    #             if target_cluster in old_clusters:
    #                 old_clusters |= merges[target_cluster]
    #                 target_clusters.remove(target_cluster)
    #                 del merges[target_cluster]
    for i in range(len(merges)):
        for target_cluster in set(merges.keys()):
            # print(target_cluster, merges[target_cluster])
            remove = False
            for new_cluster, old_clusters in list(merges.items()):
                if new_cluster == target_cluster:
                    continue
                if target_cluster in old_clusters:
                    remove = True
                    # print('1 ', new_cluster, old_clusters)
                    old_clusters.remove(target_cluster)
                    # print('2 ', new_cluster, old_clusters)
                    old_clusters |= merges[target_cluster]
                    # print('3 ', new_cluster, old_clusters)
            if remove:
                del merges[target_cluster]
    # print(merges)

    # update database
    for i in reversed(range(config.retry_count)):
        try:
            await update_news_items(pool, clustered_documents, merges)
            break
        except KeyboardInterrupt:
            print('Finishing database updates before exit...')
            await update_news_items(pool, clustered_documents, merges)
            clustering.save_state()
            raise KeyboardInterrupt

        # except (ConnectionRefusedError, asyncpg.exceptions.CannotConnectNowError, socket.gaierror):
        except (ConnectionRefusedError, socket.gaierror) as e:
            print('db error:', e)
            if i > 0:
                print('will wait %s seconds and retry' % str(config.retry_timeout))
                await asyncio.sleep(float(config.retry_timeout), loop=loop)

    if clustered_documents:
        clustering.save_state()


def set_config(cfg):
    global config
    config = cfg

async def main(config, loop=None):
    global clustering

    set_config(config)

    clustering = Clustering(state_file=config.get('state_file') or 'state/state.pickle')

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

    processed_documents2 = set()
    starting = None

    print('Clustering unclustered documents...')
    while True:
        try:
            documents = await get_unclustered_documents(pool, count=100, starting=starting, loop=loop)
            if documents:
                print('Clustering %i documents' % len(documents))
                await cluster_documents(pool, documents, loop=loop)
                starting = documents[-1].timeLastChanged
            else:
                print('No documents to process, will wait for about 5 minutes and retry')
                # wait about 5 minutes
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

    parser = argparse.ArgumentParser(description='UI Stack Clustering Service', formatter_class=argparse.ArgumentDefaultsHelpFormatter)
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
