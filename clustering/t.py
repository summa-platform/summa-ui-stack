#!/usr/bin/env python3

from collections import defaultdict



# 49 <- 50, 62, 63
# 52 <- 50, 58, 62, 63
# 53 <- 50, 58, 62, 63
# 51 <- 50, 58, 62, 63, 71
merges = defaultdict(set, {53: {50, 63}, 52: {50, 51, 53, 58, 62, 63}, 48: {50, 52, 58, 63}, 57: {48, 50, 71}, 55: {57, 50}, 54: {50, 63, 62, 55}, 51: {50, 54, 58, 62, 63}, 50: {32, 59, 23}, 49: {50, 63}})
# defaultdict(set, {49: {50, 63}})

for k,v in merges.items():
    print(k, '<-', ' '.join(map(str, v)))

print(merges)
for i in range(len(merges)):
    for target_cluster in set(merges.keys()):
        print(target_cluster, merges[target_cluster])
        remove = False
        for new_cluster, old_clusters in list(merges.items()):
            if new_cluster == target_cluster:
                continue
            if target_cluster in old_clusters:
                remove = True
                print('1 ', new_cluster, old_clusters)
                old_clusters.remove(target_cluster)
                print('2 ', new_cluster, old_clusters)
                old_clusters |= merges[target_cluster]
                print('3 ', new_cluster, old_clusters)
        if remove:
            del merges[target_cluster]
        pass

    # target_clusters = set(merges.keys())
    # for new_cluster, old_clusters in list(merges.items()):
    #     for target_cluster in list(target_clusters):
    #         if target_cluster in old_clusters:
    #             old_clusters |= merges[target_cluster]
    #             target_clusters.remove(target_cluster)
    #             del merges[target_cluster]
print(merges)
for k,v in merges.items():
    print(k, '<-', ' '.join(map(str, v)))
