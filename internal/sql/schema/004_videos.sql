-- +goose Up
ALTER TABLE videos
ADD COLUMN lift_result TEXT NOT NULL;

-- +goose Down
ALTER TABLE videos
DROP COLUMN lift_result;
