-- Reverts 1000_community. Posts are public content the authors chose to
-- publish; dropping the tables removes them.
DROP TABLE IF EXISTS community_reports;
DROP TABLE IF EXISTS community_replies;
DROP TABLE IF EXISTS community_posts;
