-- Add ContentType column to routes table
-- Date: 2026-03-02
-- Description: Add content_type field to support routing based on request content type (text/image/all)

ALTER TABLE routes ADD COLUMN content_type VARCHAR(10) DEFAULT 'all';

-- Add index for faster filtering (optional)
-- CREATE INDEX idx_routes_content_type ON routes(content_type);

-- Update existing routes to have default value
-- UPDATE routes SET content_type = 'all' WHERE content_type IS NULL;
