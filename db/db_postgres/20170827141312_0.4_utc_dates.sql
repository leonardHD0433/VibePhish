-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
-- Convert dates to UTC with timezone information
UPDATE campaigns SET created_date = created_date AT TIME ZONE 'UTC' WHERE created_date IS NOT NULL;
UPDATE campaigns SET completed_date = completed_date AT TIME ZONE 'UTC' WHERE completed_date IS NOT NULL;
UPDATE campaigns SET launch_date = launch_date AT TIME ZONE 'UTC' WHERE launch_date IS NOT NULL;
UPDATE events SET "time" = "time" AT TIME ZONE 'UTC' WHERE "time" IS NOT NULL;
UPDATE groups SET modified_date = modified_date AT TIME ZONE 'UTC' WHERE modified_date IS NOT NULL;
UPDATE templates SET modified_date = modified_date AT TIME ZONE 'UTC' WHERE modified_date IS NOT NULL;
UPDATE pages SET modified_date = modified_date AT TIME ZONE 'UTC' WHERE modified_date IS NOT NULL;
UPDATE smtp SET modified_date = modified_date AT TIME ZONE 'UTC' WHERE modified_date IS NOT NULL;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
-- Note: Timezone conversion is not easily reversible