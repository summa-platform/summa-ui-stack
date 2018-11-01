#!/usr/bin/env python3

import json, ssl, os, traceback, uuid, sys, asyncio, socket

from datetime import datetime, timedelta, timezone
from urllib.parse import urljoin

import asyncpg, aiohttp, certifi, yaml

# import config


feeds = {}


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

def store_json(filename, data):
    with open(filename, 'w') as f:
        json.dump(data, f, indent=2, ensure_ascii=True)

def load_json(filename):
    with open(filename, 'r') as f:
        return json.load(f, object_hook=Dict)


def verb(*args, **kwargs):
    print(*args, file=sys.stderr, **kwargs)

# {
#   "query": {
#     "endEpochTimeSecs": 1513839150,
#     "maxResultCount": 100,
#     "resultOffset": 0,
#     "startEpochTimeSecs": 1513777149
#   },
#   "result": {
#     "newsItemIds": [
#       "87cb1d42-fd27-4297-8bf5-f03bd4693b5f",
#     ],
#     "totalNewsItemsWithinRange": 1
#   }
# }


async def get_ids(session, url, start=None, end=None, offset=0, count=100):
    # url = urljoin(url, 'newsItems/ids')
    url = urljoin(url, 'newsItems/done-ids')
    params = Dict(maxResultCount=count, resultOffset=offset)
    if start:
        params.startEpochTimeSecs = str(start)
    if end:
        params.endEpochTimeSecs = str(end)
    for i in range(6):
        try:
            async with session.request('GET', url, params=params) as response:
                return (await response.json(loads=json_Dict))
        except KeyboardInterrupt:
            raise
        except aiohttp.client_exceptions.ContentTypeError as e:
            print('Error (will wait %i seconds): %s' % ((i+1)*10,e), file=sys.stderr)
            ee = e
            await asyncio.sleep((i+1)*10)
    raise ee
    # async with session.request('GET', url, params=params) as response:
    #     try:
    #         return (await response.json(loads=json_Dict))
    #     except aiohttp.client_exceptions.ContentTypeError as e:
    #         b = await response.text()
    #         print('URL:', url, file=sys.stderr)
    #         print('Params:', params, file=sys.stderr)
    #         print('Response Text:', file=sys.stderr)
    #         print(b, file=sys.stderr)
    #         raise

async def get_media_item(session, url, id):
    url = urljoin(url, 'mediaItems/%s' % id)
    async with session.request('GET', url) as response:
        try:
            return (await response.json(loads=json_Dict), response.status)
        except aiohttp.client_exceptions.ContentTypeError as e:
            b = await response.text()
            print('URL:', url, file=sys.stderr)
            print('Response Text:', file=sys.stderr)
            print(b, file=sys.stderr)
            raise

async def get_news_item(session, url, id):
    url = urljoin(url, 'newsItems/%s' % id)
    async with session.request('GET', url) as response:
        return (await response.json(loads=json_Dict))

async def get_last_time(pool, name='default'):
    async with pool.acquire() as conn:
        # values = await conn.fetch("""SELECT max(datetime) last_datetime FROM news_items""")
        values = await conn.fetch("""
            SELECT max(dt) last_datetime FROM
            (SELECT max(last_changed) dt FROM news_items WHERE origin = $1 UNION
            SELECT max(datetime) dt FROM entities) t;
        """, name)
        if len(values) > 0:
            # print(values[0]['last_datetime'])
            # print(values[0]['last_datetime'].timestamp())
            # print(values[0]['last_datetime'].replace(tzinfo=timezone.utc).timestamp())
            last_datetime = values[0]['last_datetime']
            if not last_datetime:
                return 
            return int(last_datetime.replace(tzinfo=timezone.utc).timestamp())+0   # to get next items only
            # return int(values[0]['last_datetime'].timestamp())

async def get_invalid_ids(pool, name='default', maxage=timedelta(seconds=2*3600)):
    async with pool.acquire() as conn:
        values = await conn.fetch("""
            SELECT id FROM bad_news_items
            WHERE datetime >= $1 AND origin = $2;
            """,
            datetime.utcnow()-maxage,
            # datetime.strptime(datetime.now()-maxage, '%Y-%m-%dT%H:%M:%S.%fZ'),
            name,
        )
        return [r['id'] for r in values]

async def clear_invalid_item(pool, id):
    async with pool.acquire() as conn:
        await conn.execute("""
            DELETE FROM bad_news_items WHERE id = $1;
            """,
            id
        )

async def clear_outdated_invalid_items(pool, maxage=timedelta(seconds=2*3600), name='default'):
    print('[%s] Removing outdated invalid items...' % name)
        # DELETE FROM bad_news_items WHERE datetime >= $1;
    async with pool.acquire() as conn:
        r = await conn.fetch("""
            WITH deleted AS (DELETE FROM bad_news_items WHERE datetime <= $1 AND origin = $2 RETURNING *) SELECT count(*) FROM deleted;
            """,
            # datetime.strptime(datetime.now()-maxage, '%Y-%m-%dT%H:%M:%S.%fZ'),
            datetime.utcnow()-maxage,
            name,
        )
        count = r[0]['count']
    print('[%s] %i outdated invalid items removed' % (name, count))

async def store_invalid_media_item(pool, id, mediaItem, responseCode, name='default'):
    async with pool.acquire() as conn:
        await conn.execute("""
            INSERT INTO bad_news_items(id, datetime, errcode, data, origin) VALUES($1, $2, $3, $4, $5)
            ON CONFLICT DO NOTHING
            """,
            id,
            # datetime.strptime(datetime.now(), '%Y-%m-%dT%H:%M:%S.%fZ'),
            datetime.utcnow(),
            responseCode,
            json.dumps(mediaItem),
            name,
        )

async def store_media_item(pool, id, mediaItem, responseCode=200, generate=0, name='default'):
    global feeds

    if responseCode != 200 or not mediaItem or not mediaItem.timeAdded or not mediaItem.id or not mediaItem.mediaItemType or not mediaItem.source:
        return False

    mediaItem.detectedTopics.sort(key=lambda item: item[1], reverse=True)

    async with pool.acquire() as conn:

        await conn.execute("""
            INSERT INTO news_items (id, type, lang, feed, story, datetime, last_changed, data, origin, cluster_id, topics, topic_weights)
            VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
            --ON CONFLICT (news_items.id) DO UPDATE
            ON CONFLICT ( id ) DO UPDATE
            SET type = $2, lang = $3, feed = $4, story = $5, datetime = $6, last_changed = $7, data = $8, origin = $9
            --WHERE id = $1;
            """,
            mediaItem.id,
            mediaItem.mediaItemType,
            mediaItem.detectedLangCode,
            mediaItem.source and mediaItem.source.id or '',
            # document.feedURL,
            '0', # mediaItem.storyId,
            datetime.strptime(mediaItem.timeAdded, '%Y-%m-%dT%H:%M:%S.%fZ'),
            datetime.strptime(mediaItem.timeLastChanged, '%Y-%m-%dT%H:%M:%S.%fZ'),
            json.dumps(mediaItem),
            name,
            None,
            [topic[0] for topic in mediaItem.detectedTopics or []],
            [float(topic[1]) if type(topic[1]) is str else topic[1] for topic in mediaItem.detectedTopics or []],
        )
        verb('.', end='', flush=True)

        # add feed
        if mediaItem.source and mediaItem.source.id and mediaItem.source.name:
            feed = feeds.get(mediaItem.source.id)   # retrieve from local feed cache
            if not feed:
                # try to add the source to feed lists in case it is a new feed
                feed = Dict(id=mediaItem.source.id, name=mediaItem.source.name, importedFrom='news_item')
                if mediaItem.isLivefeedChunk is not None:
                    feed.live = mediaItem.isLivefeedChunk
                await conn.execute("""
                    INSERT INTO feeds(id, data) VALUES($1, $2)
                    ON CONFLICT DO NOTHING
                    """,
                    feed.id,
                    json.dumps(feed)
                )
                feeds[feed.id] = feed   # store into local feed cache
                verb('.', end='', flush=True)
            elif feed.get('live') is None and mediaItem.isLivefeedChunk is not None:      # livefeed flag not set
                feed.live = mediaItem.isLivefeedChunk
                await conn.execute("""
                    UPDATE feeds
                    SET data = jsonb_set(data, '{live}', $2, true)
                    WHERE id = $1;
                    """,
                    feed.id,
                    'true' if feed.live else 'false',
                )
                verb('.', end='', flush=True)

        # print(mediaItem.namedEntities.entities.values())
        for entity in mediaItem.namedEntities.entities.values():
            # print(entity)
            if entity.baseForm:
                # do not insert without baseform
                await conn.execute("""
                    INSERT INTO entities (id, baseform, type, datetime)
                    VALUES($1, $2, $3, to_timestamp($4, 'YYYY-MM-DD HH24:MI:SS.MSZ'))
                    --VALUES($1, $2, $3, $4)
                    ON CONFLICT DO NOTHING
                    """,
                    entity.id,
                    entity.baseForm,
                    entity.type,
                    mediaItem.timeLastChanged,  # varētu būt vistuvākais laiks
                    # mediaItem.timeAdded,
                    #datetime.strptime(mediaItem.timeAdded, '%Y-%m-%dT%H:%M:%S.%fZ'),
                )
            # but allow to add relation
            await conn.execute("""
                INSERT INTO news_item_entities (news_item, entity_id, entity_baseform, datetime)
                VALUES($1, $2, $3, to_timestamp($4, 'YYYY-MM-DD HH24:MI:SS.MSZ'))
                --VALUES($1, $2, $3, $4)
                ON CONFLICT DO NOTHING
                """,
                mediaItem.id,
                entity.id,
                entity.baseForm,
                mediaItem.timeAdded,
                #datetime.strptime(mediaItem.timeAdded, '%Y-%m-%dT%H:%M:%S.%fZ'),
            )
            verb('.', end='', flush=True)

        # insert relations
        if mediaItem.relationships.mainText or mediaItem.relationships.teaser:

            new_relations = []
            if mediaItem.relationships.mainText:
                new_relations.extend(mediaItem.relationships.mainText)
            if mediaItem.relationships.teaser:
                new_relations.extend(mediaItem.relationships.teaser)

            # print('%i new relations for media item %s' % (len(new_relations), mediaItem.id))

            for relation in new_relations:
                relation_entities = set()
                for role, entities in relation.roles.items():
                    if isinstance(entities, dict):
                        relation_entities |= entities.keys()
                for entityID in relation.entities:
                    if entityID not in relation_entities:
                        # skip entity mentioned in list, but not involved into any role
                        continue
                    relations_obj = await conn.fetch("""SELECT relations FROM entities WHERE id = $1;""", entityID)

                    verb('.', end='', flush=True)

                    if len(relations_obj) > 0 and 'relations' in relations_obj[0]:
                        relations = json.loads(relations_obj[0]['relations'], object_hook=Dict)

                        # check for duplicates, if found, only append source
                        found = None
                        for r in relations:
                            if r.roles == relation.roles:
                                found = r
                                break
                        if found:
                            if not found.sources:
                                if found.source and found.source != relation.source:
                                    found.sources = [found.source, relation.source]
                                else:
                                    found.sources = [relation.source]
                            elif relation.source not in found.sources:
                                found.sources.append(relation.source)
                        else:
                            relations.append(relation)

                    else:
                        relations = [relation]

                    if not relations:
                        continue

                    # print('-->', entityID)

                    await conn.execute("""
                        UPDATE entities SET relations = $2 WHERE id = $1;
                        """,
                        entityID,
                        json.dumps(relations),
                    )
                    verb('.', end='', flush=True)


    return True

async def pull_id(pool, session, url, id, name='default', invalid=False, generate=0):
    mediaItem, responseCode = await get_media_item(session, url, id)
    if responseCode != 200 or not mediaItem or not mediaItem.timeAdded or not mediaItem.id or not mediaItem.mediaItemType or not mediaItem.source:
        return False
    verb('.', end='', flush=True)
    if generate == 0:
        r = await store_media_item(pool, id, mediaItem, responseCode, generate=generate, name=name)
        if invalid and r:
            await clear_invalid_item(pool, id)
            verb('.', end='', flush=True)
        elif not invalid and not r:
            # TODO: error storing not getting from origin, should be marked as such
            # await store_invalid_media_item(conn, id, mediaItem, responseCode, generate=generate, name=name)
            # verb('.', end='', flush=True)
            pass
        return r
    elif generate > 0:
        originalID = mediaItem.id
        originalTitle = Dict(mediaItem.title)
        verb('[%i]' % generate, end='', flush=True)
        for i in range(generate):
            for k,v in originalTitle.items():
                mediaItem.title[k] = '%s (#%i)' % (v, i+1)
                mediaItem.id = str(uuid.uuid4())    # generate new id
            verb('\n  ->', mediaItem.id, end=' ', flush=True)
            # print(mediaItem.title, mediaItem.id)
            r = await store_media_item(pool, mediaItem.id, mediaItem, responseCode, generate=generate, name=name)
            if invalid and r:
                await clear_invalid_item(pool, id)  # originalID
                verb('.', end='', flush=True)
            elif not invalid and not r:
                # TODO: error storing not getting from origin, should be marked as such
                # await store_invalid_media_item(conn, id, mediaItem, responseCode, generate=generate, name=name)    # originalID
                # verb('.', end='', flush=True)
                pass
            if not r:
                return r
        return r
    # return await store_media_item(conn, id, mediaItem, responseCode)
    # try:
    #     return await store_media_item(conn, id, mediaItem)
    # except KeyboardInterrupt:
    #     # allow to finish
    #     await store_media_item(conn, id, mediaItem)
    #     raise

async def pull_ids(pool, session, url, ids, name='default', invalid=False, generate=0, verbose=False):
    print('[%s] Pulling data for %i %sids' % (name, len(ids), 'INVALID ' if invalid else ''))
    for id in ids:
        if verbose:
            print('[%s] PULL %s ' % (name, id), end='', flush=True)
        else:
            print('[%s] PULL %s ...' % (name, id))
        r = False
        # for i in range(10):
        while True:
            i = 0
            try:
                # mediaItem = await get_media_item(session, id)
                # try:
                #     done = await store_media_item(conn, id, mediaItem)
                # except KeyboardInterrupt:
                #     done = await store_media_item(conn, id, mediaItem)
                r = await pull_id(pool, session, url, id, name=name, invalid=invalid, generate=generate)
                if verbose:
                    print(r and ' OK' or ' INVALID')
                else:
                    print('[%s] PULL %s ... OK' % (name, id))
                break
            except KeyboardInterrupt:
                # allow to finish
                # print(await pull_id(conn, session, id, invalid=invalid) and ' OK' or ' INVALID')
                raise
            except Exception as e:
                if verbose:
                    print(' FAIL')
                else:
                    print('[%s] PULL %s ... FAIL: %s: %s ; will retry' % (name, id, e.__class__.__name__, e))
                await asyncio.sleep((i+1)*60)   # wait 1, 2, 3, ..., 10 minutes
                # traceback.print_exc()
        if not r and not invalid:
            # TODO: store failure in db for later retrieval
            print('[%s] PULL %s ... TOO MANY FAILS, STORING AS INVALID' % (name, id))
            await store_invalid_media_item(pool, id, mediaItem=dict(), responseCode=0, name=name)    # originalID
    print('[%s] Batch done' % name)

async def pull_invalid_ids(pool, session, url, name='default', verbose=False):
    ids = await get_invalid_ids(pool, name=name)
    await pull_ids(pool, session, url, ids, name=name, invalid=True, verbose=verbose)

async def pull(pool, url, name='default', start=None, offset=0, generate=0, batch_size=500, verbose=False):
    # conn = await asyncpg.connect(**config.db)
    try:
        sslcontext = ssl.create_default_context(cafile=certifi.where())
        async with aiohttp.ClientSession(connector=aiohttp.TCPConnector(ssl=sslcontext), read_timeout=20*60, conn_timeout=30) as session:

            if start is None:
                start = await get_last_time(pool, name=name)
            # offset = 0
            total = 0
            end = None

            last_invalid_items_cleanup = datetime.now()
            last_invalid_items_retry = datetime.now()
            response = None

            while True:

                response = None
                print('[%s] Fetching id batch: start=%s%s, end=%s%s, offset=%i, total=%i' % (name,
                    start, ' (%s)' % datetime.fromtimestamp(start).strftime('%Y-%m-%d %H:%M:%S UTC') if start else '',
                    end, ' (%s)' % datetime.fromtimestamp(end).strftime('%Y-%m-%d %H:%M:%S UTC') if end else '',
                    offset, total))
                while not response:
                    try:
                        response = await get_ids(session, url, start=start, end=end, offset=offset, count=batch_size)
                        if not response:
                            raise Exception('invalid response: empty response') # shouldn't happen
                        if not response.get('result'):
                            raise Exception("invalid response: missing field 'result'")
                        if not response.get('query'):
                            raise Exception("invalid response: missing field 'query'")
                        total = response.result.totalNewsItemsWithinRange
                        start = response.query.startEpochTimeSecs
                        end = response.query.endEpochTimeSecs
                        ids = response.result.newsItemIds
                    except KeyboardInterrupt:
                        raise
                    except Exception as e:
                        response = None     # reset response, so it won't break the loop on next try
                        print('[%s] Error fetching id batch:' % name, e)
                        print('[%s] Wait a minute to retry...' % name)
                        await asyncio.sleep(60)
                print('[%s] Got id batch: start=%s%s, end=%s%s, offset=%i, total=%i, count=%i' % (name,
                    start, ' (%s)' % datetime.fromtimestamp(start).strftime('%Y-%m-%d %H:%M:%S UTC') if start else '',
                    end, ' (%s)' % datetime.fromtimestamp(end).strftime('%Y-%m-%d %H:%M:%S UTC') if end else '',
                    offset, total, len(ids)))
                # print('epoch', datetime.utcnow().timestamp())
                # print('epoch', datetime.now().timestamp())

                if len(ids) > 0:
                    await pull_ids(pool, session, url, ids, name=name, generate=generate, verbose=verbose)
                    offset += len(ids)

                if offset >= total or len(ids) == 0:
                    offset = 0
                    start = end
                    end = None
                    print('[%s] Nothing to pull, wait a minute...' % name)
                    await asyncio.sleep(60)
                    # TODO: retry downloading incomplete items

                # retry invalid items
                d = datetime.now() - last_invalid_items_retry
                if d.days > 0 or d.seconds > 20*60:    # retry each 20 minutes
                    await pull_invalid_ids(pool, session, url, name=name, verbose=verbose)
                    last_invalid_items_retry = datetime.now()

                # clear outdated invalid items
                d = datetime.now() - last_invalid_items_cleanup
                if d.days > 0 or d.seconds > 00*2:    # cleanup each 2 hours
                    await clear_outdated_invalid_items(pool, name=name)
                    last_invalid_items_cleanup = datetime.now()


    # except KeyboardInterrupt:
    #     print('INTERRUPTED')
    finally:
        # await conn.close()
        pass

async def run(args, loop=None):
    global feeds
    # conn = await asyncpg.connect(**config.db)
    # pool = await asyncpg.create_pool(**config.db)
    # print('Connecting to database')
    print('Connecting to database', file=sys.stderr)
    while True:
        try:
            pool = await asyncpg.create_pool(
                    user=config.db.user,
                    password=config.db.password,
                    port=config.db.port or 5432,
                    host=config.db.host or 'localhost',
                    database=config.db.dbname,
                    server_settings=dict(application_name=config.db.app_name or 'summa-ui-pull'),
                    min_size=1,
                    max_size=config.db.pool_max_connections or 3,
            )
            break
        except (ConnectionRefusedError, asyncpg.exceptions.CannotConnectNowError, socket.gaierror):
            print('Error connecting, will reconnect', file=sys.stderr)
            await asyncio.sleep(config.db.reconnect_sleep or 10)

    async with pool.acquire() as conn:

        origins = {}
        rows = await conn.fetch("""
            SELECT name, url, video_url FROM origins;
        """)
        for row in rows:
            name = row['name']
            url = row['url']
            video_url = row['video_url'] or url
            origins[name] = (url, video_url)

        if args.list:
            print('Sources in database:')
            for key in sorted(origins.keys()):
                print('%s = %s' % (key, '% ; %s' % origins[key]))

        if args.reset:
            print('Reset: removing all previous sources')
            await conn.execute("""
                DELETE FROM origins;
            """)
            origins = {}
        elif args.rm:
            for key in args.rm:
                print('Removing source with key:', key)
                if key in origins:
                    del origins[key]
            await conn.execute("""
                DELETE FROM origins WHERE name = ANY($1);
            """, args.rm)

        db_origins = dict(origins)
        if args.selected:
            origins = {}

        for i, source in enumerate(args.source):
            if source.startswith('http://') or source.startswith('https://'):
                source = source.split(',')
                url = source[0] if source[0].endswith('/') else source[0]+'/'
                if len(source) > 1:
                    video_url = source[1] if source[1].endswith('/') else source[1]+'/'
                else:
                    video_url = url
                name = 'default%i' % i
            else:
                kv = source.split('=', 1)
                if len(kv) == 2:
                    name, urls = kv
                    urls = urls.split(',')
                    url = urls[0] if urls[0].endswith('/') else urls[0]+'/'
                    if len(urls) > 1:
                        video_url = urls[1] if urls[1].endswith('/') else urls[1]+'/'
                    else:
                        video_url = url
                elif len(kv) == 1:
                    name = source
                    urls = origins.get(name)
                    if not url:
                        print("error: origin/source %s not found in database, please specify URL" % name, file=sys.stderr)
                        sys.exit(1)
                        return
                    url, video_url = urls
            origins[name] = (url, video_url)
        
        futures = []
        for name, urls in origins.items():
            url, video_url = urls
            if name not in db_origins:
                print('Adding source: %s = %s' % (name, '%s ; %s' % urls))
            elif url != db_origins[name][0] or video_url != db_origins[name][1]:
                print('Updating source: %s = %s' % (name, '%s ; %s' % urls))
            await conn.execute("""
                INSERT INTO origins (name, url, video_url) VALUES($1, $2, $3)
                ON CONFLICT ON CONSTRAINT unique_origins_name DO UPDATE SET url = $2, video_url = $3 WHERE origins.name = $1
                """,
                name,
                url,
                video_url,
            )
            if not args.exit:
                print('Source %s : %s' % (name, '%s ; %s' % urls))
                futures.append(pull(pool, url, name=name, start=args.start, offset=args.offset, generate=args.generate, verbose=args.verbose))

        # get all feeds
        rows = await conn.fetch("""
            SELECT id, data FROM feeds;
        """)
        for row in rows:
            id = row['id']
            feeds[id] = json.loads(row['data'], object_hook=Dict)

    if args.exit:
        return
    # wait for all futures
    if futures:
        done, pending = await asyncio.wait(futures, loop=loop)
    else:
        print('Error: no pull tasks defined, ran with arguments:', ' '.join(sys.argv), file=sys.stderr)
        sys.exit(1)

# TODO: recover from asyncpg.exceptions.ConnectionDoesNotExistError


def load_config(filename='config.yaml'):

    # https://www.programcreek.com/python/example/11269/yaml.add_constructor
    # https://pyyaml.org/wiki/PyYAMLDocumentation
    # https://stackoverflow.com/questions/36629559/how-to-use-custom-dictionary-class-while-loading-yaml

    def dict_constructor(loader, node):
        return Dict(loader.construct_mapping(node))

    yaml.add_constructor(yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, dict_constructor)

    with open(filename, 'r') as f:
        config = yaml.load(f)

    return config



if __name__ == "__main__":
    # TODO: ability to pull from multiple sources, possibly select source as command line argument

    import argparse

    parser = argparse.ArgumentParser(description='Data Puller', formatter_class=argparse.ArgumentDefaultsHelpFormatter)
    parser.add_argument('--config', '-c', metavar='FILE', type=str, default='config.yaml', help='database configuration file')
    parser.add_argument('--selected', '-x', action='store_true', help='pull only selected sources/origins')
    parser.add_argument('--verbose', '-v', action='store_true', help='verbose mode')
    parser.add_argument('--reset', '-r', action='store_true', help='clear list of origins/sources')
    parser.add_argument('--rm', type=str, metavar='KEY', action='append', help='remove source by key')
    parser.add_argument('--list', '-l', action='store_true', help='list origins/sources in database')
    parser.add_argument('--exit', action='store_true', help='do not pull, update list of origins/sources and exit')
    parser.add_argument('--start', '-s', type=str, default=None, help='specify start timestamp (epoch or YYYYmmddTHH:MM:...)')
    parser.add_argument('--offset', '-o', type=int, default=0, help='specify offset')
    parser.add_argument('--generate', '-g', metavar='N', type=int, default=0,
            help='instead of original media item, generate N modified copies (id & title)')
    parser.add_argument('source', type=str, nargs='*',
            help='specify one or more source NAME=URL[;VIDEO_URL pair], where NAME is an identifier for the given source and will be used internally')

    args = parser.parse_args()

    if not args.verbose:
        verb = lambda *args, **kwargs: None

    config = load_config(args.config or 'config.yaml')

    # other option:
    loop = asyncio.get_event_loop()
    # loop.run_until_complete(run())
    try:
        loop.run_until_complete(run(args))
        # loop.run_until_complete(pull(start=args.start, generate=args.gen))
    except KeyboardInterrupt:
        print('INTERRUPTED')
