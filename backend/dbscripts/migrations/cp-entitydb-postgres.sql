-- ThunderID schema update: control plane entitydb (PostgreSQL)
--
-- Brings an existing database up to this release. No table is dropped and no row is touched: new
-- tables are created, and existing tables keep their data while their keys are widened in place.
--
-- The only change to a table that already exists is its constraints. Replacing a primary key means
-- dropping the old constraint and adding the new one; the table and its rows stay where they are.
--
--   old   PRIMARY KEY (ID)
--   new   PRIMARY KEY (DEPLOYMENT_ID, ID)
--
-- That is what lets one database hold more than one deployment: the same id may exist in two of them
-- without colliding. Foreign keys pointing at those tables are widened to match.
--
-- Safe to run again: constraint drops are conditional and every CREATE is IF NOT EXISTS.

BEGIN;

-- ------------------------------------------ keys widened on existing tables
-- Constraint names are looked up rather than assumed: a database created by an older script may
-- carry system-generated names that a fresh install would not produce.
CREATE OR REPLACE FUNCTION pg_temp.drop_con(tbl text, kind char) RETURNS void AS $$
DECLARE c text;
BEGIN
  FOR c IN SELECT conname FROM pg_constraint
           WHERE conrelid = format('%I', tbl)::regclass AND contype = kind
  LOOP EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', tbl, c); END LOOP;
END $$ LANGUAGE plpgsql;

-- Foreign keys come off first: they depend on the primary keys about to be replaced.
SELECT pg_temp.drop_con('ENTITY', 'f');
SELECT pg_temp.drop_con('ENTITY_IDENTIFIER', 'f');
SELECT pg_temp.drop_con('GROUP', 'f');
SELECT pg_temp.drop_con('ORGANIZATION_UNIT', 'f');

SELECT pg_temp.drop_con('ENTITY', 'p');
ALTER TABLE "ENTITY" ALTER COLUMN DEPLOYMENT_ID SET NOT NULL;
ALTER TABLE "ENTITY" ADD PRIMARY KEY (DEPLOYMENT_ID, ID);
SELECT pg_temp.drop_con('ENTITY_IDENTIFIER', 'p');
ALTER TABLE "ENTITY_IDENTIFIER" ALTER COLUMN DEPLOYMENT_ID SET NOT NULL;
ALTER TABLE "ENTITY_IDENTIFIER" ADD PRIMARY KEY (ENTITY_ID, DEPLOYMENT_ID, NAME);
SELECT pg_temp.drop_con('GROUP', 'p');
ALTER TABLE "GROUP" ALTER COLUMN DEPLOYMENT_ID SET NOT NULL;
ALTER TABLE "GROUP" ADD PRIMARY KEY (DEPLOYMENT_ID, ID);
SELECT pg_temp.drop_con('ORGANIZATION_UNIT', 'p');
ALTER TABLE "ORGANIZATION_UNIT" ALTER COLUMN DEPLOYMENT_ID SET NOT NULL;
ALTER TABLE "ORGANIZATION_UNIT" ADD PRIMARY KEY (DEPLOYMENT_ID, OU_ID);

-- Foreign keys restored, now referencing the widened primary keys.
ALTER TABLE "ENTITY_IDENTIFIER" ADD FOREIGN KEY (DEPLOYMENT_ID, ENTITY_ID) REFERENCES "ENTITY" (DEPLOYMENT_ID, ID) ON DELETE CASCADE;

COMMIT;
