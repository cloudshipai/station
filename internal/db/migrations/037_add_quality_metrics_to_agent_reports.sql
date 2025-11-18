-- Migration: Add quality metrics to agent report details
-- Enables PDF reports to display LLM-as-judge evaluation scores

-- +goose Up
ALTER TABLE agent_report_details ADD COLUMN quality_metrics TEXT;  -- JSON: QualityMetrics with avg scores and pass rates

-- +goose Down
ALTER TABLE agent_report_details DROP COLUMN quality_metrics;
