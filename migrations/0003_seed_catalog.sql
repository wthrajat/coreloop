INSERT OR IGNORE INTO topics
    (id, slug, title, lane, difficulty, prerequisites_json, objectives_json)
VALUES
    ('topic_backend_http', 'http-production', 'HTTP in Production', 'Backend engineering', 'intermediate', '["Basic networking","A backend language"]', '["Explain the HTTP request lifecycle","Choose status codes and caching safely","Design idempotent APIs","Debug production failures"]'),
    ('topic_backend_db', 'database-systems', 'Database Systems', 'Backend engineering', 'intermediate', '["SQL basics"]', '["Explain storage and query execution","Design indexes from workloads","Use transactions deliberately","Recognize consistency trade-offs"]'),
    ('topic_backend_dist', 'distributed-systems', 'Distributed Systems', 'Backend engineering', 'intermediate', '["HTTP","Databases"]', '["Reason about partial failure","Design idempotent workflows","Choose consistency boundaries","Operate queues and retries"]'),
    ('topic_cloud_foundations', 'cloud-foundations', 'Cloud Engineering Foundations', 'Cloud engineering', 'intermediate', '["Linux basics","Networking basics"]', '["Map cloud primitives to system needs","Design secure deployments","Estimate operational cost","Build observable services"]'),
    ('topic_cloud_iac', 'infrastructure-as-code', 'Infrastructure as Code', 'Cloud engineering', 'beginner', '["Cloud foundations"]', '["Explain why declarative infrastructure exists","Use Terraform safely","Manage state and drift","Review infrastructure changes"]'),
    ('topic_ai_models', 'ai-model-foundations', 'AI Model Foundations', 'Applied AI', 'beginner', '["Programming experience"]', '["Explain tokens and inference","Select models by measured constraints","Understand context and caching","Recognize model failure modes"]'),
    ('topic_ai_systems', 'production-ai-systems', 'Production AI Systems', 'Applied AI', 'intermediate', '["AI model foundations","Backend engineering"]', '["Build reliable model workflows","Evaluate quality and cost","Use structured outputs safely","Design fallbacks and observability"]'),
    ('topic_product', 'product-engineering', 'Product Engineering', 'Product engineering', 'intermediate', '["Software delivery experience"]', '["Turn problems into testable outcomes","Choose scope from evidence","Connect technical decisions to users","Operate feedback loops"]'),
    ('topic_security', 'application-security', 'Application Security', 'Engineering fundamentals', 'intermediate', '["HTTP","Databases"]', '["Model threats","Protect identity and sessions","Handle secrets and untrusted input","Design least-privilege systems"]'),
    ('topic_reliability', 'reliability-engineering', 'Reliability Engineering', 'Engineering fundamentals', 'intermediate', '["Backend engineering"]', '["Define service objectives","Design graceful degradation","Use telemetry to diagnose failures","Run incident learning loops"]'),
    ('topic_communication', 'technical-communication', 'Technical Communication', 'Career leverage', 'intermediate', '[]', '["Write clear engineering decisions","Explain systems in interviews","Communicate risk and trade-offs","Align teams around outcomes"]'),
    ('topic_sales', 'technical-sales', 'Technical Sales for Engineers', 'Career leverage', 'beginner', '[]', '["Discover user pain","Explain value without hype","Qualify product opportunities","Handle objections with evidence"]'),
    ('topic_bitcoin', 'bitcoin-engineering', 'Bitcoin Engineering', 'Bitcoin and protocols', 'intermediate', '["Programming experience"]', '["Explain Bitcoin from first principles","Build reliable protocol integrations","Evaluate real utility and trade-offs","Separate evidence from market narratives"]');

INSERT OR IGNORE INTO sources
    (id, publisher, canonical_url, source_tier, fetch_method, trust_notes, polling_interval_minutes)
VALUES
    ('source_go_blog', 'The Go Blog', 'https://go.dev/blog/feed.atom', 1, 'atom', 'Official Go project publication', 360),
    ('source_github_changelog', 'GitHub Changelog', 'https://github.blog/changelog/feed/', 1, 'rss', 'Official GitHub product changelog', 180),
    ('source_kubernetes_blog', 'Kubernetes Blog', 'https://kubernetes.io/feed.xml', 1, 'rss', 'Official Kubernetes project publication', 360),
    ('source_cloudflare_blog', 'Cloudflare Blog', 'https://blog.cloudflare.com/rss/', 1, 'rss', 'Official Cloudflare engineering publication', 360),
    ('source_aws_news', 'AWS What''s New', 'https://aws.amazon.com/about-aws/whats-new/recent/feed/', 1, 'rss', 'Official AWS product announcements', 360);

INSERT OR IGNORE INTO prompt_versions
    (id, prompt_version, schema_version, compiler_version, instruction_checksum, schema_checksum, evaluation_status, approved_at)
VALUES
    ('prompt_lesson_v1', 'lesson-v1', 'lesson-draft-v1', 'compiler-v1', 'runtime-verified', 'runtime-verified', 'approved', strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));
