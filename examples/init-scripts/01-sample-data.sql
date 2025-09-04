-- Sample data initialization for Enhanced PostgreSQL
-- This script demonstrates the capabilities of the enhanced PostgreSQL distribution

-- Create sample schema for a blog application
CREATE SCHEMA IF NOT EXISTS blog;

-- Create users table with PostgREST-compatible structure
CREATE TABLE blog.users (
    id SERIAL PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    username TEXT UNIQUE NOT NULL,
    full_name TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    is_active BOOLEAN DEFAULT true,
    -- Vector embedding for user preferences (using pgvector)
    preferences_embedding vector(768)
);

-- Create posts table
CREATE TABLE blog.posts (
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    content TEXT,
    excerpt TEXT,
    author_id INTEGER REFERENCES blog.users(id) ON DELETE CASCADE,
    published BOOLEAN DEFAULT false,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    -- Tags stored as JSON
    tags JSONB DEFAULT '[]'::jsonb,
    -- Content embedding for similarity search
    content_embedding vector(768),
    -- Full text search vector
    search_vector tsvector
);

-- Create comments table
CREATE TABLE blog.comments (
    id SERIAL PRIMARY KEY,
    post_id INTEGER REFERENCES blog.posts(id) ON DELETE CASCADE,
    author_id INTEGER REFERENCES blog.users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    is_approved BOOLEAN DEFAULT false
);

-- Create categories table
CREATE TABLE blog.categories (
    id SERIAL PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Create many-to-many relationship between posts and categories
CREATE TABLE blog.post_categories (
    post_id INTEGER REFERENCES blog.posts(id) ON DELETE CASCADE,
    category_id INTEGER REFERENCES blog.categories(id) ON DELETE CASCADE,
    PRIMARY KEY (post_id, category_id)
);

-- Create analytics table for metrics
CREATE TABLE blog.analytics (
    id SERIAL PRIMARY KEY,
    event_type TEXT NOT NULL,
    event_data JSONB,
    user_id INTEGER REFERENCES blog.users(id),
    session_id TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    -- Additional metadata
    user_agent TEXT,
    ip_address INET,
    referrer TEXT
);

-- Create indexes for performance
CREATE INDEX idx_posts_author ON blog.posts(author_id);
CREATE INDEX idx_posts_published ON blog.posts(published, published_at);
CREATE INDEX idx_posts_search_vector ON blog.posts USING gin(search_vector);
CREATE INDEX idx_posts_tags ON blog.posts USING gin(tags);
CREATE INDEX idx_comments_post ON blog.comments(post_id);
CREATE INDEX idx_analytics_event_type ON blog.analytics(event_type);
CREATE INDEX idx_analytics_created_at ON blog.analytics(created_at);

-- Create function to update search vector automatically
CREATE OR REPLACE FUNCTION blog.update_search_vector() RETURNS trigger AS $$
BEGIN
    NEW.search_vector := to_tsvector('english', COALESCE(NEW.title, '') || ' ' || COALESCE(NEW.content, ''));
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger for search vector updates
CREATE TRIGGER update_post_search_vector
    BEFORE INSERT OR UPDATE ON blog.posts
    FOR EACH ROW EXECUTE FUNCTION blog.update_search_vector();

-- Create function to update timestamps
CREATE OR REPLACE FUNCTION blog.update_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create triggers for automatic timestamp updates
CREATE TRIGGER update_users_timestamp
    BEFORE UPDATE ON blog.users
    FOR EACH ROW EXECUTE FUNCTION blog.update_updated_at();

CREATE TRIGGER update_posts_timestamp
    BEFORE UPDATE ON blog.posts
    FOR EACH ROW EXECUTE FUNCTION blog.update_updated_at();

CREATE TRIGGER update_comments_timestamp
    BEFORE UPDATE ON blog.comments
    FOR EACH ROW EXECUTE FUNCTION blog.update_updated_at();

-- Insert sample categories
INSERT INTO blog.categories (name, slug, description) VALUES
    ('Technology', 'technology', 'Posts about technology and programming'),
    ('AI & Machine Learning', 'ai-ml', 'Artificial intelligence and machine learning topics'),
    ('Database', 'database', 'Database design, optimization, and management'),
    ('Web Development', 'web-dev', 'Frontend and backend web development'),
    ('DevOps', 'devops', 'Development operations and infrastructure');

-- Insert sample users
INSERT INTO blog.users (email, username, full_name, preferences_embedding) VALUES
    ('alice@example.com', 'alice', 'Alice Johnson', '[0.1, 0.2, 0.3]'::vector),
    ('bob@example.com', 'bob', 'Bob Smith', '[0.4, 0.5, 0.6]'::vector),
    ('charlie@example.com', 'charlie', 'Charlie Brown', '[0.7, 0.8, 0.9]'::vector);

-- Insert sample posts with realistic content
WITH sample_posts AS (
    SELECT 
        'Getting Started with pgvector' as title,
        'PostgreSQL vector extension opens up new possibilities for AI applications. In this post, we explore how to use pgvector for similarity search and recommendation systems.' as content,
        'Learn how to use pgvector for AI applications in PostgreSQL' as excerpt,
        1 as author_id,
        '["postgresql", "ai", "vector-search"]'::jsonb as tags,
        '[0.1, 0.15, 0.2, 0.25, 0.3]'::vector as content_embedding
    UNION ALL
    SELECT 
        'Advanced PgBouncer Configuration',
        'Connection pooling is crucial for high-performance PostgreSQL applications. This guide covers advanced PgBouncer configuration patterns for production deployments.',
        'Master PgBouncer configuration for production PostgreSQL deployments',
        2,
        '["postgresql", "pgbouncer", "performance"]'::jsonb,
        '[0.2, 0.25, 0.3, 0.35, 0.4]'::vector
    UNION ALL
    SELECT 
        'Building REST APIs with PostgREST',
        'PostgREST automatically generates REST APIs from your PostgreSQL schema. This tutorial shows you how to build secure, performant APIs with zero code.',
        'Create REST APIs from PostgreSQL with PostgREST',
        3,
        '["postgresql", "postgrest", "api"]'::jsonb,
        '[0.3, 0.35, 0.4, 0.45, 0.5]'::vector
)
INSERT INTO blog.posts (title, content, excerpt, author_id, tags, content_embedding, published, published_at)
SELECT title, content, excerpt, author_id, tags, content_embedding, true, NOW() - interval '1 day' * (3 - author_id)
FROM sample_posts;

-- Link posts to categories
INSERT INTO blog.post_categories (post_id, category_id) VALUES
    (1, 2), (1, 3), -- pgvector post -> AI/ML, Database
    (2, 3), (2, 5), -- PgBouncer post -> Database, DevOps
    (3, 3), (3, 4); -- PostgREST post -> Database, Web Development

-- Insert sample comments
INSERT INTO blog.comments (post_id, author_id, content, is_approved) VALUES
    (1, 2, 'Great introduction to pgvector! This will be very helpful for my ML project.', true),
    (1, 3, 'Do you have any examples with real vector data?', true),
    (2, 1, 'The performance improvements are impressive. We saw 50% reduction in connection overhead.', true),
    (3, 2, 'PostgREST is amazing. We replaced 500 lines of API code with just schema definitions.', true);

-- Insert sample analytics data
INSERT INTO blog.analytics (event_type, event_data, user_id, session_id, user_agent, ip_address) VALUES
    ('page_view', '{"page": "/posts/1", "duration": 45}', 1, 'sess_123', 'Mozilla/5.0', '192.168.1.100'),
    ('page_view', '{"page": "/posts/2", "duration": 60}', 2, 'sess_456', 'Mozilla/5.0', '192.168.1.101'),
    ('comment_posted', '{"post_id": 1, "comment_id": 1}', 2, 'sess_456', 'Mozilla/5.0', '192.168.1.101'),
    ('search', '{"query": "pgvector tutorial", "results": 3}', 3, 'sess_789', 'Mozilla/5.0', '192.168.1.102');

-- Create a view for published posts with author information
CREATE VIEW blog.published_posts AS
SELECT 
    p.id,
    p.title,
    p.excerpt,
    p.published_at,
    p.tags,
    u.username as author_username,
    u.full_name as author_name,
    array_agg(c.name) as categories
FROM blog.posts p
JOIN blog.users u ON p.author_id = u.id
LEFT JOIN blog.post_categories pc ON p.id = pc.post_id
LEFT JOIN blog.categories c ON pc.category_id = c.id
WHERE p.published = true
GROUP BY p.id, p.title, p.excerpt, p.published_at, p.tags, u.username, u.full_name
ORDER BY p.published_at DESC;

-- Create function for vector similarity search
CREATE OR REPLACE FUNCTION blog.find_similar_posts(query_embedding vector, similarity_threshold float DEFAULT 0.8, max_results int DEFAULT 10)
RETURNS TABLE(
    id integer,
    title text,
    excerpt text,
    similarity float
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        p.id,
        p.title,
        p.excerpt,
        1 - (p.content_embedding <-> query_embedding) as similarity
    FROM blog.posts p
    WHERE p.published = true 
      AND p.content_embedding IS NOT NULL
      AND (1 - (p.content_embedding <-> query_embedding)) >= similarity_threshold
    ORDER BY p.content_embedding <-> query_embedding
    LIMIT max_results;
END;
$$ LANGUAGE plpgsql;

-- Create function for full-text search
CREATE OR REPLACE FUNCTION blog.search_posts(search_query text)
RETURNS TABLE(
    id integer,
    title text,
    excerpt text,
    rank float
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        p.id,
        p.title,
        p.excerpt,
        ts_rank(p.search_vector, plainto_tsquery('english', search_query)) as rank
    FROM blog.posts p
    WHERE p.published = true 
      AND p.search_vector @@ plainto_tsquery('english', search_query)
    ORDER BY rank DESC;
END;
$$ LANGUAGE plpgsql;

-- Schedule a cleanup job using pg_cron
SELECT cron.schedule('cleanup-old-analytics', '0 2 * * *', 
    'DELETE FROM blog.analytics WHERE created_at < NOW() - interval ''90 days''');

-- Grant appropriate permissions for PostgREST
GRANT USAGE ON SCHEMA blog TO anon, authenticated;
GRANT SELECT ON ALL TABLES IN SCHEMA blog TO anon;
GRANT SELECT, INSERT, UPDATE ON blog.comments TO authenticated;
GRANT SELECT, INSERT ON blog.analytics TO authenticated;
GRANT SELECT, INSERT, UPDATE, DELETE ON blog.posts TO authenticated;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA blog TO authenticated;

-- Grant permissions on views and functions
GRANT SELECT ON blog.published_posts TO anon;
GRANT EXECUTE ON FUNCTION blog.find_similar_posts(vector, float, int) TO anon;
GRANT EXECUTE ON FUNCTION blog.search_posts(text) TO anon;

-- Create a simple health check for the blog schema
CREATE OR REPLACE FUNCTION blog.health_check()
RETURNS json
LANGUAGE sql
STABLE
AS $$
    SELECT json_build_object(
        'status', 'healthy',
        'schema', 'blog',
        'tables', (SELECT count(*) FROM information_schema.tables WHERE table_schema = 'blog'),
        'users', (SELECT count(*) FROM blog.users),
        'posts', (SELECT count(*) FROM blog.posts),
        'comments', (SELECT count(*) FROM blog.comments),
        'last_post', (SELECT published_at FROM blog.posts WHERE published = true ORDER BY published_at DESC LIMIT 1)
    );
$$;

GRANT EXECUTE ON FUNCTION blog.health_check() TO anon;

-- Add some sample metrics to the metrics table for demonstration
INSERT INTO public.metrics (name, value) VALUES
    ('total_users', (SELECT count(*) FROM blog.users)),
    ('published_posts', (SELECT count(*) FROM blog.posts WHERE published = true)),
    ('total_comments', (SELECT count(*) FROM blog.comments)),
    ('approved_comments', (SELECT count(*) FROM blog.comments WHERE is_approved = true));

-- Create a notification function (useful with pg_notify)
CREATE OR REPLACE FUNCTION blog.notify_new_comment()
RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('new_comment', 
        json_build_object(
            'post_id', NEW.post_id,
            'comment_id', NEW.id,
            'author_id', NEW.author_id
        )::text
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER notify_new_comment
    AFTER INSERT ON blog.comments
    FOR EACH ROW EXECUTE FUNCTION blog.notify_new_comment();

-- Final message
DO $$
BEGIN
    RAISE NOTICE 'Enhanced PostgreSQL sample data loaded successfully!';
    RAISE NOTICE 'Available endpoints via PostgREST:';
    RAISE NOTICE '  GET /blog.published_posts - View published posts';
    RAISE NOTICE '  GET /blog.users - View users';
    RAISE NOTICE '  GET /blog.categories - View categories';
    RAISE NOTICE '  POST /rpc/blog.find_similar_posts - Vector similarity search';
    RAISE NOTICE '  POST /rpc/blog.search_posts - Full-text search';
    RAISE NOTICE '  GET /rpc/blog.health_check - Schema health check';
END;
$$;