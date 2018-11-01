ALTER TABLE "public"."users" ADD COLUMN "data" JSONB DEFAULT '{}'::jsonb NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS "users_index_email_lower" ON "public"."users" USING btree( lower(email) Asc NULLS Last );
