#!/usr/bin/env python3

import os, sys, traceback
import signal

import asyncio
from asyncio.subprocess import create_subprocess_exec, PIPE

import aiohttp


from summarization_service import load_config, run as summarization_service


server_proc = None
clustering_server_cmd = os.path.join(os.path.dirname(__file__), 'summarization_server.sh')


def log(*args, **kwargs):
    print(*args, file=sys.stderr, **kwargs)


async def run_server(loop=None):
    global server_proc
    log('Starting Priberam Summarization Server ...')
    # ignore SIGINT in child process to allow to execute safe shutdown
    # https://stackoverflow.com/a/13737455
    def preexec_fn():
        # signal.signal(signal.SIGINT, signal.SIG_IGN) 
        pass
    server_proc = await create_subprocess_exec(clustering_server_cmd, loop=loop, preexec_fn=preexec_fn)
    # server_proc = await create_subprocess_exec(clustering_server_cmd, stdin=PIPE, stdout=PIPE, loop=loop)
    # result, stderr = await asyncio.wait_for(server_proc.communicate(text), timeout=timeout)
    await server_proc.wait()
    return server_proc.returncode



if __name__ == "__main__":

    import argparse

    parser = argparse.ArgumentParser(description='UI Stack Cluster Summarization Service Entrypoint', formatter_class=argparse.ArgumentDefaultsHelpFormatter)
    parser.add_argument('--config', '-c', metavar='FILE', type=str, default='config.yaml', help='database configuration file')
    parser.add_argument('--verbose', '-v', action='store_true', help='verbose mode')

    args = parser.parse_args()

    config = load_config(args.config)

    try:
        loop = asyncio.get_event_loop()

        asyncio.ensure_future(run_server(loop=loop))

        loop.run_until_complete(summarization_service(config, verbose=args.verbose))

    except KeyboardInterrupt:
        print('INTERRUPTED')
    except:
        print('EXCEPTION')
        traceback.print_exc()
