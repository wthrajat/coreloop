package store

import (
	"context"
	"time"
)

type RadarEnrichment struct {
	Summary, WhyItMatters, Provider, Model string
}

func (store *Store) RadarEnrichment(
	ctx context.Context,
	sourceItemID string,
	inputHash string,
	version string,
) (RadarEnrichment, error) {
	var enrichment RadarEnrichment
	err := store.database.QueryRowContext(ctx, `SELECT summary,why_it_matters,provider,model
		FROM radar_enrichments WHERE source_item_id=? AND input_hash=? AND enrichment_version=?`,
		sourceItemID, inputHash, version).
		Scan(&enrichment.Summary, &enrichment.WhyItMatters, &enrichment.Provider, &enrichment.Model)
	return enrichment, err
}

func (store *Store) SaveRadarEnrichment(
	ctx context.Context,
	sourceItemID string,
	inputHash string,
	version string,
	enrichment RadarEnrichment,
	now time.Time,
) error {
	_, err := store.database.ExecContext(ctx, `INSERT INTO radar_enrichments
		(source_item_id,input_hash,enrichment_version,summary,why_it_matters,provider,model,created_at)
		VALUES (?,?,?,?,?,?,?,?) ON CONFLICT(source_item_id,input_hash,enrichment_version) DO NOTHING`,
		sourceItemID, inputHash, version, enrichment.Summary, enrichment.WhyItMatters,
		enrichment.Provider, enrichment.Model, timestamp(now))
	return err
}
