-- https://gis.stackexchange.com/questions/247113/how-to-properly-set-up-indexes-for-postgis-distance-queries/247131
CREATE EXTENSION postgis;

ALTER TABLE "public"."entities" ADD COLUMN "geo" GEOGRAPHY NULL;
ALTER TABLE "public"."entities" DROP CONSTRAINT geo_enforce_srid;
ALTER TABLE "public"."entities" ADD CONSTRAINT entities_geo_enforce_srid CHECK (st_srid(geo) = 4326);
CREATE INDEX index_entities_geo ON entities USING GIST (geography(geo));
