ALTER TABLE learning_preferences ADD COLUMN radar_items_per_day INTEGER NOT NULL DEFAULT 8
    CHECK (radar_items_per_day BETWEEN 0 AND 50);
ALTER TABLE learning_preferences ADD COLUMN radar_weekends_enabled INTEGER NOT NULL DEFAULT 1
    CHECK (radar_weekends_enabled IN (0, 1));

ALTER TABLE sources ADD COLUMN source_role TEXT NOT NULL DEFAULT 'primary'
    CHECK (source_role IN ('primary', 'technical_editorial', 'community_discovery', 'community_discussion'));
ALTER TABLE sources ADD COLUMN adapter_config_json TEXT NOT NULL DEFAULT '{}'
    CHECK (json_valid(adapter_config_json));

ALTER TABLE source_items ADD COLUMN normalized_url TEXT;
ALTER TABLE source_items ADD COLUMN category TEXT NOT NULL DEFAULT 'engineering'
    CHECK (category IN ('research', 'release', 'security', 'funding', 'partnership', 'pricing', 'industry', 'discussion', 'product_update', 'engineering'));
ALTER TABLE source_items ADD COLUMN community_score INTEGER NOT NULL DEFAULT 0 CHECK (community_score >= 0);
ALTER TABLE source_items ADD COLUMN community_comments INTEGER NOT NULL DEFAULT 0 CHECK (community_comments >= 0);
ALTER TABLE source_items ADD COLUMN community_signals_available INTEGER NOT NULL DEFAULT 0
    CHECK (community_signals_available IN (0, 1));
ALTER TABLE source_items ADD COLUMN discovery_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(discovery_json));

UPDATE source_items
SET normalized_url=canonical_url
WHERE normalized_url IS NULL OR normalized_url='';

ALTER TABLE radar_candidates ADD COLUMN released_at TEXT;

CREATE TABLE radar_deliveries (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    candidate_id TEXT NOT NULL UNIQUE REFERENCES radar_candidates(id) ON DELETE CASCADE,
    destination_id TEXT NOT NULL REFERENCES delivery_destinations(id) ON DELETE RESTRICT,
    job_id TEXT REFERENCES job_queue(id) ON DELETE SET NULL,
    state TEXT NOT NULL DEFAULT 'pending'
        CHECK (state IN ('pending', 'sending', 'delivered', 'partial', 'failed')),
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    started_at TEXT,
    completed_at TEXT,
    last_error_code TEXT,
    last_error_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE radar_delivery_parts (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    delivery_id TEXT NOT NULL REFERENCES radar_deliveries(id) ON DELETE CASCADE,
    sequence_number INTEGER NOT NULL CHECK (sequence_number > 0),
    rendered_text TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'pending'
        CHECK (state IN ('pending', 'delivered', 'failed')),
    telegram_message_id TEXT,
    sent_at TEXT,
    last_error_code TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (delivery_id, sequence_number)
);

CREATE TABLE radar_daily_usage (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    local_date TEXT NOT NULL,
    released_count INTEGER NOT NULL DEFAULT 0 CHECK (released_count >= 0),
    last_released_at TEXT,
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (user_id, local_date)
);

CREATE TABLE radar_enrichments (
    source_item_id TEXT NOT NULL REFERENCES source_items(id) ON DELETE CASCADE,
    input_hash TEXT NOT NULL,
    enrichment_version TEXT NOT NULL,
    summary TEXT NOT NULL,
    why_it_matters TEXT NOT NULL,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (source_item_id, input_hash, enrichment_version)
);

CREATE INDEX source_items_normalized_url_idx ON source_items(normalized_url);
CREATE INDEX radar_candidates_release_idx
    ON radar_candidates(user_id, status, relevance_score DESC, created_at);
CREATE INDEX radar_candidates_released_at_idx
    ON radar_candidates(user_id, released_at);
CREATE INDEX radar_deliveries_user_state_idx
    ON radar_deliveries(user_id, state, created_at);

UPDATE sources
SET source_role='primary', adapter_config_json='{"adapter":"feed"}'
WHERE id IN ('source_go_blog', 'source_github_changelog', 'source_kubernetes_blog', 'source_cloudflare_blog', 'source_aws_news');

INSERT OR IGNORE INTO sources
    (id, publisher, canonical_url, source_tier, fetch_method, trust_notes,
     polling_interval_minutes, source_role, adapter_config_json)
VALUES
    ('source_openai_news', 'OpenAI News', 'https://openai.com/news/rss.xml', 1, 'rss',
     'Official OpenAI news feed', 180, 'primary', '{"adapter":"feed"}'),
    ('source_anthropic', 'Anthropic', 'https://www.anthropic.com/sitemap.xml', 1, 'html',
     'Official Anthropic news and research pages discovered from its sitemap', 240, 'primary',
     '{"adapter":"sitemap","item_limit":12,"path_prefixes":["/news/","/research/"]}'),
    ('source_deepseek', 'DeepSeek', 'https://api-docs.deepseek.com/sitemap.xml', 1, 'html',
     'Official DeepSeek news pages discovered from its documentation sitemap', 240, 'primary',
     '{"adapter":"sitemap","item_limit":12,"path_prefixes":["/news/"]}'),
    ('source_huggingface_blog', 'Hugging Face Blog', 'https://huggingface.co/blog/feed.xml', 1, 'rss',
     'Official Hugging Face publication', 240, 'primary', '{"adapter":"feed"}'),
    ('source_hacker_news', 'Hacker News', 'https://hacker-news.firebaseio.com/v0/beststories.json', 3, 'api',
     'Official Hacker News API used for community discovery and discussion', 60, 'community_discovery',
     '{"adapter":"hacker_news","item_limit":30}'),
    ('source_stacker_news', 'Stacker News', 'https://stacker.news/rss', 3, 'rss',
     'Stacker News front-page RSS used for Bitcoin and technology discovery', 90, 'community_discovery',
     '{"adapter":"feed"}'),
    ('source_stacker_bitcoin', 'Stacker News · Bitcoin', 'https://stacker.news/~bitcoin/rss', 3, 'rss',
     'Stacker News Bitcoin territory RSS', 90, 'community_discovery', '{"adapter":"feed"}'),
    ('source_stacker_lightning', 'Stacker News · Lightning', 'https://stacker.news/~lightning/rss', 3, 'rss',
     'Stacker News Lightning territory RSS', 90, 'community_discovery', '{"adapter":"feed"}'),
    ('source_stacker_tech', 'Stacker News · Tech', 'https://stacker.news/~tech/rss', 3, 'rss',
     'Stacker News technology territory RSS', 90, 'community_discovery', '{"adapter":"feed"}'),
    ('source_rust_blog', 'Rust Blog', 'https://blog.rust-lang.org/feed.xml', 1, 'rss',
     'Official Rust project publication', 360, 'primary', '{"adapter":"feed"}'),
    ('source_node_blog', 'Node.js Blog', 'https://nodejs.org/en/feed/blog.xml', 1, 'rss',
     'Official Node.js project publication', 360, 'primary', '{"adapter":"feed"}'),
    ('source_python_blog', 'Python Insider', 'https://blog.python.org/feeds/posts/default', 1, 'rss',
     'Official Python release publication', 360, 'primary', '{"adapter":"feed"}'),
    ('source_postgresql_news', 'PostgreSQL News', 'https://www.postgresql.org/sitemap.xml', 1, 'html',
     'Official PostgreSQL project news discovered from its sitemap', 360, 'primary',
     '{"adapter":"sitemap","item_limit":12,"path_prefixes":["/about/news/"]}'),
    ('source_google_cloud', 'Google Cloud Release Notes', 'https://cloud.google.com/feeds/gcp-release-notes.xml', 1, 'rss',
     'Official Google Cloud product release notes', 240, 'primary', '{"adapter":"feed"}'),
    ('source_azure_updates', 'Azure SDK Blog', 'https://devblogs.microsoft.com/azure-sdk/feed/', 1, 'rss',
     'Official Microsoft Azure SDK engineering publication', 240, 'primary', '{"adapter":"feed"}'),
    ('source_cisa_advisories', 'CISA Advisories', 'https://www.cisa.gov/cybersecurity-advisories/all.xml', 1, 'rss',
     'Official United States cybersecurity advisories', 120, 'primary', '{"adapter":"feed"}'),
    ('source_arxiv_ai', 'arXiv · Artificial Intelligence', 'https://export.arxiv.org/rss/cs.AI', 2, 'rss',
     'arXiv artificial-intelligence research discovery', 360, 'technical_editorial', '{"adapter":"feed"}'),
    ('source_arxiv_ml', 'arXiv · Machine Learning', 'https://export.arxiv.org/rss/cs.LG', 2, 'rss',
     'arXiv machine-learning research discovery', 360, 'technical_editorial', '{"adapter":"feed"}'),
    ('source_bitcoin_core', 'Bitcoin Core', 'https://bitcoincore.org/en/rss.xml', 1, 'rss',
     'Official Bitcoin Core project feed', 240, 'primary', '{"adapter":"feed"}'),
    ('source_bitcoin_optech', 'Bitcoin Optech', 'https://bitcoinops.org/feed.xml', 2, 'rss',
     'Technical Bitcoin and Lightning operations publication', 240, 'technical_editorial', '{"adapter":"feed"}'),
    ('source_bitcoin_releases', 'Bitcoin Core Releases', 'https://api.github.com/repos/bitcoin/bitcoin/releases', 1, 'api',
     'Official Bitcoin Core GitHub releases', 360, 'primary', '{"adapter":"github_releases"}'),
    ('source_lnd_releases', 'LND Releases', 'https://api.github.com/repos/lightningnetwork/lnd/releases', 1, 'api',
     'Official LND GitHub releases', 360, 'primary', '{"adapter":"github_releases"}'),
    ('source_cln_releases', 'Core Lightning Releases', 'https://api.github.com/repos/ElementsProject/lightning/releases', 1, 'api',
     'Official Core Lightning GitHub releases', 360, 'primary', '{"adapter":"github_releases"}'),
    ('source_ldk_releases', 'LDK Releases', 'https://api.github.com/repos/lightningdevkit/rust-lightning/releases', 1, 'api',
     'Official Lightning Development Kit GitHub releases', 360, 'primary', '{"adapter":"github_releases"}'),
    ('source_bolts_releases', 'Lightning BOLTs Releases', 'https://api.github.com/repos/lightning/bolts/releases', 1, 'api',
     'Official Lightning specification GitHub releases', 720, 'primary', '{"adapter":"github_releases"}'),
    ('source_deepmind_blog', 'Google DeepMind', 'https://deepmind.google/blog/rss.xml', 1, 'rss',
     'Official Google DeepMind research and product publication', 240, 'primary', '{"adapter":"feed"}'),
    ('source_netflix_tech', 'Netflix Technology Blog', 'https://netflixtechblog.com/feed', 2, 'rss',
     'Official Netflix engineering publication', 360, 'technical_editorial', '{"adapter":"feed"}'),
    ('source_slack_engineering', 'Slack Engineering', 'https://slack.engineering/feed/', 2, 'rss',
     'Official Slack engineering publication', 360, 'technical_editorial', '{"adapter":"feed"}'),
    ('source_github_engineering', 'GitHub Engineering', 'https://github.blog/engineering/feed/', 2, 'rss',
     'Official GitHub engineering publication', 360, 'technical_editorial', '{"adapter":"feed"}'),
    ('source_meta_engineering', 'Engineering at Meta', 'https://engineering.fb.com/feed/', 2, 'rss',
     'Official Meta engineering publication', 360, 'technical_editorial', '{"adapter":"feed"}'),
    ('source_mozilla_hacks', 'Mozilla Hacks', 'https://hacks.mozilla.org/feed/', 2, 'rss',
     'Official Mozilla publication for web developers', 360, 'technical_editorial', '{"adapter":"feed"}'),
    ('source_cncf_blog', 'CNCF Blog', 'https://www.cncf.io/feed/', 2, 'rss',
     'Official Cloud Native Computing Foundation publication', 360, 'technical_editorial', '{"adapter":"feed"}'),
    ('source_lobsters', 'Lobsters', 'https://lobste.rs/rss', 3, 'rss',
     'Curated computing community used for technical discovery and discussion', 90, 'community_discovery', '{"adapter":"feed"}'),
    ('source_anthropic_bluesky', 'Anthropic on Bluesky', 'https://public.api.bsky.app/xrpc/app.bsky.feed.getAuthorFeed?actor=anthropic.com&limit=30&filter=posts_no_replies', 3, 'api',
     'Verified Anthropic Bluesky account used for discovery', 120, 'community_discovery',
     '{"adapter":"bluesky_author","item_limit":30}'),
    ('source_github_bluesky', 'GitHub on Bluesky', 'https://public.api.bsky.app/xrpc/app.bsky.feed.getAuthorFeed?actor=github.com&limit=30&filter=posts_no_replies', 3, 'api',
     'Verified GitHub Bluesky account used for discovery', 120, 'community_discovery',
     '{"adapter":"bluesky_author","item_limit":30}'),
    ('source_microsoft_devblogs', 'Microsoft Developer Blogs', 'https://devblogs.microsoft.com/feed/', 2, 'rss',
     'Official Microsoft engineering and developer publication', 240, 'technical_editorial', '{"adapter":"feed"}');
