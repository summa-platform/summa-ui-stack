--
-- PostgreSQL database dump
--

-- Dumped from database version 10.2
-- Dumped by pg_dump version 10.2

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET check_function_bodies = false;
SET client_min_messages = warning;
SET row_security = off;

SET search_path = public, pg_catalog;

SET default_tablespace = '';

SET default_with_oids = false;

--
-- Name: bad_news_items; Type: TABLE; Schema: public; Owner: summa
--

CREATE TABLE bad_news_items (
    id text NOT NULL,
    datetime timestamp without time zone NOT NULL,
    errcode integer NOT NULL,
    data jsonb NOT NULL,
    origin text DEFAULT 'default'::text NOT NULL
);


ALTER TABLE bad_news_items OWNER TO summa;

--
-- Name: bookmarks; Type: TABLE; Schema: public; Owner: summa
--

CREATE TABLE bookmarks (
    id text NOT NULL,
    datetime timestamp without time zone NOT NULL,
    type text NOT NULL,
    "user" text NOT NULL,
    title text NOT NULL,
    item jsonb NOT NULL
);


ALTER TABLE bookmarks OWNER TO summa;

--
-- Name: entities; Type: TABLE; Schema: public; Owner: summa
--

CREATE TABLE entities (
    id text NOT NULL,
    baseform text NOT NULL,
    type text NOT NULL,
    datetime timestamp without time zone DEFAULT now() NOT NULL,
    relations jsonb DEFAULT '[]'::jsonb NOT NULL
);


ALTER TABLE entities OWNER TO summa;

--
-- Name: feed_group_feeds; Type: TABLE; Schema: public; Owner: summa
--

CREATE TABLE feed_group_feeds (
    feed_group text NOT NULL,
    feed text NOT NULL
);


ALTER TABLE feed_group_feeds OWNER TO summa;

--
-- Name: feed_groups; Type: TABLE; Schema: public; Owner: summa
--

CREATE TABLE feed_groups (
    id text NOT NULL,
    name text NOT NULL
);


ALTER TABLE feed_groups OWNER TO summa;

--
-- Name: feeds; Type: TABLE; Schema: public; Owner: summa
--

CREATE TABLE feeds (
    id text NOT NULL,
    data jsonb NOT NULL
);


ALTER TABLE feeds OWNER TO summa;

--
-- Name: news_item_entities; Type: TABLE; Schema: public; Owner: summa
--

CREATE TABLE news_item_entities (
    news_item text NOT NULL,
    entity_baseform text NOT NULL,
    entity_id text NOT NULL,
    datetime timestamp without time zone
);


ALTER TABLE news_item_entities OWNER TO summa;

--
-- Name: news_items; Type: TABLE; Schema: public; Owner: summa
--

CREATE TABLE news_items (
    id text NOT NULL,
    type text NOT NULL,
    feed text NOT NULL,
    story text NOT NULL,
    datetime timestamp without time zone,
    lang text,
    data jsonb,
    origin text DEFAULT 'default'::text NOT NULL,
    last_changed timestamp without time zone DEFAULT now() NOT NULL
);


ALTER TABLE news_items OWNER TO summa;

--
-- Name: origins; Type: TABLE; Schema: public; Owner: summa
--

CREATE TABLE origins (
    name text NOT NULL,
    url text NOT NULL
);


ALTER TABLE origins OWNER TO summa;

--
-- Name: queries; Type: TABLE; Schema: public; Owner: summa
--

CREATE TABLE queries (
    id text NOT NULL,
    name text DEFAULT ''::text NOT NULL,
    "user" text NOT NULL,
    feed_groups jsonb DEFAULT '[]'::jsonb NOT NULL,
    entities jsonb DEFAULT '[]'::jsonb NOT NULL,
    data jsonb DEFAULT '{}'::jsonb NOT NULL,
    datetime timestamp without time zone DEFAULT timezone('utc'::text, now()) NOT NULL
);


ALTER TABLE queries OWNER TO summa;

--
-- Name: reports; Type: TABLE; Schema: public; Owner: summa
--

CREATE TABLE reports (
    id text NOT NULL,
    datetime timestamp without time zone NOT NULL,
    data jsonb NOT NULL
);


ALTER TABLE reports OWNER TO summa;

--
-- Name: stories; Type: TABLE; Schema: public; Owner: summa
--

CREATE TABLE stories (
    id text NOT NULL,
    title text NOT NULL,
    summary text NOT NULL,
    data jsonb NOT NULL
);


ALTER TABLE stories OWNER TO summa;

--
-- Name: users; Type: TABLE; Schema: public; Owner: summa
--

CREATE TABLE users (
    id text NOT NULL,
    email text NOT NULL,
    name text NOT NULL,
    role text NOT NULL,
    suspended boolean NOT NULL,
    password text
);


ALTER TABLE users OWNER TO summa;

--
-- Name: bad_news_items bad_news_items_unique_id; Type: CONSTRAINT; Schema: public; Owner: summa
--

ALTER TABLE ONLY bad_news_items
    ADD CONSTRAINT bad_news_items_unique_id UNIQUE (id);


--
-- Name: bookmarks bookmarks_unique_id; Type: CONSTRAINT; Schema: public; Owner: summa
--

ALTER TABLE ONLY bookmarks
    ADD CONSTRAINT bookmarks_unique_id UNIQUE (id);


--
-- Name: entities entities_unique_id; Type: CONSTRAINT; Schema: public; Owner: summa
--

ALTER TABLE ONLY entities
    ADD CONSTRAINT entities_unique_id PRIMARY KEY (id);


--
-- Name: news_items news_items_pkey; Type: CONSTRAINT; Schema: public; Owner: summa
--

ALTER TABLE ONLY news_items
    ADD CONSTRAINT news_items_pkey PRIMARY KEY (id);


--
-- Name: queries queries_unique_id; Type: CONSTRAINT; Schema: public; Owner: summa
--

ALTER TABLE ONLY queries
    ADD CONSTRAINT queries_unique_id PRIMARY KEY (id);


--
-- Name: reports reports_unique_id; Type: CONSTRAINT; Schema: public; Owner: summa
--

ALTER TABLE ONLY reports
    ADD CONSTRAINT reports_unique_id UNIQUE (id);


--
-- Name: stories stories_unique_id; Type: CONSTRAINT; Schema: public; Owner: summa
--

ALTER TABLE ONLY stories
    ADD CONSTRAINT stories_unique_id PRIMARY KEY (id);


--
-- Name: feed_group_feeds unique_feed_group_feeds; Type: CONSTRAINT; Schema: public; Owner: summa
--

ALTER TABLE ONLY feed_group_feeds
    ADD CONSTRAINT unique_feed_group_feeds UNIQUE (feed_group, feed);


--
-- Name: feed_groups unique_feed_group_id; Type: CONSTRAINT; Schema: public; Owner: summa
--

ALTER TABLE ONLY feed_groups
    ADD CONSTRAINT unique_feed_group_id UNIQUE (id);


--
-- Name: feeds unique_id; Type: CONSTRAINT; Schema: public; Owner: summa
--

ALTER TABLE ONLY feeds
    ADD CONSTRAINT unique_id PRIMARY KEY (id);


--
-- Name: news_item_entities unique_news_item_entity; Type: CONSTRAINT; Schema: public; Owner: summa
--

ALTER TABLE ONLY news_item_entities
    ADD CONSTRAINT unique_news_item_entity UNIQUE (news_item, entity_baseform, entity_id);


--
-- Name: origins unique_origins_name; Type: CONSTRAINT; Schema: public; Owner: summa
--

ALTER TABLE ONLY origins
    ADD CONSTRAINT unique_origins_name UNIQUE (name);


--
-- Name: users unique_users_email; Type: CONSTRAINT; Schema: public; Owner: summa
--

ALTER TABLE ONLY users
    ADD CONSTRAINT unique_users_email UNIQUE (email);


--
-- Name: users unique_users_id; Type: CONSTRAINT; Schema: public; Owner: summa
--

ALTER TABLE ONLY users
    ADD CONSTRAINT unique_users_id UNIQUE (id);


--
-- Name: bookmarks_index_datetime; Type: INDEX; Schema: public; Owner: summa
--

CREATE INDEX bookmarks_index_datetime ON bookmarks USING btree (datetime);


--
-- Name: bookmarks_index_user; Type: INDEX; Schema: public; Owner: summa
--

CREATE INDEX bookmarks_index_user ON bookmarks USING btree ("user");


--
-- Name: entities_index_datetime; Type: INDEX; Schema: public; Owner: summa
--

CREATE INDEX entities_index_datetime ON entities USING btree (datetime);


--
-- Name: index_bad_news_items_datetime; Type: INDEX; Schema: public; Owner: summa
--

CREATE INDEX index_bad_news_items_datetime ON bad_news_items USING btree (datetime);


--
-- Name: index_bad_news_items_origin; Type: INDEX; Schema: public; Owner: summa
--

CREATE INDEX index_bad_news_items_origin ON bad_news_items USING btree (origin);


--
-- Name: index_entities_baseform_lower; Type: INDEX; Schema: public; Owner: summa
--

CREATE INDEX index_entities_baseform_lower ON entities USING btree (lower(baseform));


--
-- Name: index_feed_group_feeds_feed; Type: INDEX; Schema: public; Owner: summa
--

CREATE INDEX index_feed_group_feeds_feed ON feed_group_feeds USING btree (feed);


--
-- Name: index_feed_group_feeds_feed_group; Type: INDEX; Schema: public; Owner: summa
--

CREATE INDEX index_feed_group_feeds_feed_group ON feed_group_feeds USING btree (feed_group);


--
-- Name: index_news_item_entities_baseform_lower; Type: INDEX; Schema: public; Owner: summa
--

CREATE INDEX index_news_item_entities_baseform_lower ON news_item_entities USING btree (lower(entity_baseform));


--
-- Name: index_news_item_entities_datetime; Type: INDEX; Schema: public; Owner: summa
--

CREATE INDEX index_news_item_entities_datetime ON news_item_entities USING btree (datetime);


--
-- Name: index_news_item_entities_entity_baseform; Type: INDEX; Schema: public; Owner: summa
--

CREATE INDEX index_news_item_entities_entity_baseform ON news_item_entities USING btree (entity_baseform);


--
-- Name: index_news_item_entities_entity_id; Type: INDEX; Schema: public; Owner: summa
--

CREATE INDEX index_news_item_entities_entity_id ON news_item_entities USING btree (entity_id);


--
-- Name: index_news_item_entities_news_item; Type: INDEX; Schema: public; Owner: summa
--

CREATE INDEX index_news_item_entities_news_item ON news_item_entities USING btree (news_item);


--
-- Name: index_news_items_english_fts; Type: INDEX; Schema: public; Owner: summa
--

CREATE INDEX index_news_items_english_fts ON news_items USING gin (((((setweight(to_tsvector('english'::regconfig, COALESCE(((data -> 'title'::text) ->> 'english'::text), ''::text)), 'A'::"char") || setweight(to_tsvector('english'::regconfig, COALESCE(((data -> 'teaser'::text) ->> 'english'::text), (((data -> 'transcript'::text) -> 'english'::text) ->> 'text'::text), ''::text)), 'B'::"char")) || setweight(to_tsvector('english'::regconfig, COALESCE(((data -> 'mainText'::text) ->> 'english'::text), ''::text)), 'C'::"char")) || setweight(to_tsvector('english'::regconfig, COALESCE((data ->> 'summary'::text), ''::text)), 'D'::"char"))));


--
-- Name: index_news_items_feed_url; Type: INDEX; Schema: public; Owner: summa
--

CREATE INDEX index_news_items_feed_url ON news_items USING btree (feed);


--
-- Name: index_news_items_lang; Type: INDEX; Schema: public; Owner: summa
--

CREATE INDEX index_news_items_lang ON news_items USING btree (lang);


--
-- Name: index_news_items_last_changed; Type: INDEX; Schema: public; Owner: summa
--

CREATE INDEX index_news_items_last_changed ON news_items USING btree (last_changed);


--
-- Name: index_news_items_origin; Type: INDEX; Schema: public; Owner: summa
--

CREATE INDEX index_news_items_origin ON news_items USING btree (origin);


--
-- Name: index_news_items_story_id; Type: INDEX; Schema: public; Owner: summa
--

CREATE INDEX index_news_items_story_id ON news_items USING btree (story);


--
-- Name: index_news_items_time_added; Type: INDEX; Schema: public; Owner: summa
--

CREATE INDEX index_news_items_time_added ON news_items USING btree (datetime);


--
-- Name: index_news_items_type; Type: INDEX; Schema: public; Owner: summa
--

CREATE INDEX index_news_items_type ON news_items USING btree (type);


--
-- Name: queries_index_datetime; Type: INDEX; Schema: public; Owner: summa
--

CREATE INDEX queries_index_datetime ON queries USING btree (datetime);


--
-- Name: reports_index_datetime; Type: INDEX; Schema: public; Owner: summa
--

CREATE INDEX reports_index_datetime ON reports USING btree (datetime);


--
-- PostgreSQL database dump complete
--

