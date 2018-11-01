-- CLUSTERS

-- CREATE FIELD "cluster_id" -----------------------------------
ALTER TABLE "public"."news_items" ADD COLUMN "cluster_id" Integer;
-- -------------------------------------------------------------

-- -- CREATE FIELD "cluster_bucket_id" ----------------------------
-- ALTER TABLE "public"."news_items" ADD COLUMN "cluster_bucket_id" Integer;
-- -- -------------------------------------------------------------

-- -- CREATE INDEX "index_news_items_cluster_span_id" -------------
-- CREATE INDEX "index_news_items_cluster_span_id" ON "public"."news_items" USING btree( "cluster_bucket_id" );
-- -- -------------------------------------------------------------

-- CREATE INDEX "index_news_items_cluster_id" ------------------
CREATE INDEX "index_news_items_cluster_id" ON "public"."news_items" USING btree( "cluster_id" );
-- -------------------------------------------------------------


-- CREATE TABLE "cross_lingual_clusters" -----------------------
CREATE TABLE "public"."cross_lingual_clusters" ( 
	"cluster_id" Integer NOT NULL,
	"cross_lingual_cluster_id" Integer NOT NULL );
-- -------------------------------------------------------------

-- CREATE INDEX "cross_lingual_cluster_index" -----------------
CREATE INDEX "cross_lingual_cluster_index" ON "public"."cross_lingual_clusters" USING btree( "cluster_id" Asc NULLS Last, "cross_lingual_cluster_id" Asc NULLS Last );
-- -------------------------------------------------------------


-- CHANGE "UNIQUE" OF "FIELD "cluster_id" ----------------------
-- Will be changed by uniques
-- -------------------------------------------------------------

-- CHANGE "INDEXED" OF "FIELD "cross_lingual_cluster_id" -------
-- Will be changed by indexes
-- -------------------------------------------------------------


-- CREATE INDEX "cross_lingual_clusters_index_cross_lingual_cluster_id" 
CREATE INDEX "cross_lingual_clusters_index_cross_lingual_cluster_id" ON "public"."cross_lingual_clusters" USING btree( "cross_lingual_cluster_id" );
-- -------------------------------------------------------------

-- CREATE UNIQUE "unique_cross_lingual_clusters_cluster_id" ----
ALTER TABLE "public"."cross_lingual_clusters" ADD CONSTRAINT "unique_cross_lingual_clusters_cluster_id" UNIQUE( "cluster_id" );
-- -------------------------------------------------------------


-- TOPICS

ALTER TABLE news_items ADD COLUMN topics TEXT[] DEFAULT '{}'::TEXT[] NOT NULL;

CREATE INDEX news_items_index_topics ON news_items USING GIN (topics);

ALTER TABLE news_items ADD COLUMN topic_weights DOUBLE PRECISION[] DEFAULT '{}'::DOUBLE PRECISION[] NOT NULL;
