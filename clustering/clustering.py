#!/usr/bin/env python3

import re, os.path, pickle
from collections import defaultdict, namedtuple

from gensim import corpora, models


scriptdir = os.path.dirname(__file__)

dic = None
tfidf = None

def load_dictionary():
    global dic
    dic = corpora.Dictionary.load(os.path.join(scriptdir, 'models/american-local-news.lower.150000_min=2_below=0.3_nostop.dic'))


def load_tfidf_model():
    global tfidf
    tfidf = models.tfidfmodel.TfidfModel.load(os.path.join(scriptdir, 'models/american-local-news.lower.tfidf'))


def init():
    load_dictionary()
    load_tfidf_model()


def get_features(text):
    global dic, tfidf

    # lazy initialization
    if not dic or not tfidf:
        init()

    text_norm = ' '.join(re.split(r'\W+', text)).lower().strip()

    bow = dic.doc2bow(text_norm.split())
    bow = tfidf[bow]
    bow_top = sorted(bow, key=lambda x: x[1], reverse=True)[:100]

    features = {k:v for k,v in bow_top}

    return features


def distance(a, b):
    dividend = 0
    quotient = 0

    for i in a:
        dividend += a[i]
        if i in b:
            quotient += abs(a[i] - b[i])
        else:
            quotient += a[i]

    for i in b:
        dividend += b[i]
        if i not in a:
            quotient += b[i]

    return quotient / dividend



class Clustering:

    Response = namedtuple('Response', 'cluster, merged')

    def __init__(self, state_file='state.pickle'):
        self.documents = {}
        self.clusters = defaultdict(set)
        self.next_cluster = 0
        self.state_file = state_file
        self.load_state()

    def clear(self):
        self.documents = {}
        self.clusters = defaultdict(set)
        self.save_state()

    def load_state(self):
        if os.path.isfile(self.state_file):
            try:
                print('Loading state from', self.state_file)
                with open(self.state_file, 'rb') as infile:
                    state = pickle.load(infile)
                    self.clusters = state['clusters']
                    self.documents = state['documents']
                    self.next_cluster = state['next_cluster']
            except:
                pass

    def save_state(self):
        data_dir = os.path.dirname(self.state_file)
        if data_dir and not os.path.isdir(data_dir):
            os.makedirs(data_dir)
        with open(self.state_file, 'wb') as outfile:
            pickle.dump(dict(documents=self.documents, clusters=self.clusters, next_cluster=self.next_cluster), outfile)

    def get_similar_clusters(self, a):
        documents = self.documents
        similar = set()

        for _,document in documents.items():
            d = distance(a['topics'], document['topics'])

            if d <= 0.678:
                similar.add(document['cluster'])

        return similar

    def set_cluster(self, old_cluster, new_cluster):
        clusters = self.clusters
        documents = self.documents

        clusters[new_cluster].update(clusters[old_cluster])
        for doc_id in clusters[old_cluster]:
            documents[doc_id]['cluster'] = new_cluster
            # document = documents.get(doc_id)
            # if document:
            #     document['cluster'] = new_cluster

        del clusters[old_cluster]   # this creates a bug later in the code, where len(clusters) is used

    def add(self, document):
        # print('ADD', document['id'])

        clusters = self.clusters
        documents = self.documents

        if not document:
            return

        if document['id'] in documents:
            return self.Response(cluster=documents[document['id']]['cluster'], merged=[])

        document['topics'] = get_features(document['text'])

        similar = self.get_similar_clusters(document)

        if len(similar) == 1:
            cluster = similar.pop()
        else:
            cluster = self.next_cluster
            self.next_cluster += 1
            # cluster = len(clusters) # len(clusters) is not correct cluster counter as del clusters[...] is used to remove old clusters

        clusters[cluster].add(document['id'])
        document['cluster'] = cluster

        for item in similar:
            self.set_cluster(item, cluster)

        documents[document['id']] = document

        # save state each 100 documents
        # if len(documents) % 100 == 0:
        #     self.save_state()

        return self.Response(cluster=cluster, merged=list(c for c in similar if c != cluster))



def test():
    clustering = Clustering()
    print("Add new document, returns cluster 0")
    print(clustering.add({
		"id": 1,
		"text": "Russia"
    }))
    print()
    print("If only one similar cluster exists, will return that cluster id, 0 in this case")
    print(clustering.add({
		"id": 11,
		"text": "Russia"
    }))
    print()
    print("Add different document, returns cluster 1")
    print(clustering.add({
		"id": 2,
		"text": "Obama"
    }))
    print()
    print("Will create new cluster 2, merging clusters 0 and 1")
    print(clustering.add({
		"id": 3,
		"text": "Obama in Russia"
    }))
    print()
    print("The same thing again will return cluster 2, because ID exists")
    print(clustering.add({
		"id": 1,
		"text": "Russia"
    }))
    print()
    print("The same thing from previous cluster 0 will return cluster 2 because of merge")
    print(clustering.add({
		"id": 5,
		"text": "Russia"
    }))
    print()
    print("Clear the database")
    clustering.clear()
    print()
    print("Now the same thing will return 0")
    print(clustering.add({
		"id": 1,
		"text": "Russia"
    }))
    print()



if __name__ == '__main__':
    test()
