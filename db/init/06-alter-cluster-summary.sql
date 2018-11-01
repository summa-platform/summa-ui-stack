
-- CREATE TABLE "clusters" -------------------------------------
CREATE TABLE "public"."clusters" ( 
	"id" Integer NOT NULL,
	"highlights" JSONB,
	"highlights_source_count" Integer,
	"highlights_datetime" Timestamp Without Time Zone,
	"title" Text,
	CONSTRAINT "clusters_unique_id" UNIQUE( "id" ) );
 ;
-- -------------------------------------------------------------

-- CREATE INDEX "clusters_index_highlights_datetime" -----------
CREATE INDEX "clusters_index_highlights_datetime" ON "public"."clusters" USING btree( "highlights_datetime" );
-- -------------------------------------------------------------
