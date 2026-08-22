package scraper

import (
	"context"
	"time"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

// FeedCheckpointStore persists Faz 20 RSS/Atom polling state: the conditional
// GET validators for a feed, and which items have already been observed (and
// with what content hash), so a restart does not re-notify on unchanged
// entries. RSSFeedSource depends on it; other adapters ignore it.
type FeedCheckpointStore interface {
	LoadFeedCheckpoint(ctx context.Context, sourceKey string) (domain.FeedCheckpoint, bool, error)
	SaveFeedCheckpoint(ctx context.Context, checkpoint domain.FeedCheckpoint) error
	LoadSeenFeedItem(ctx context.Context, sourceKey, itemKey string) (contentHash string, found bool, err error)
	MarkSeenFeedItem(ctx context.Context, sourceKey, itemKey, contentHash string, seenAt time.Time) error
}
