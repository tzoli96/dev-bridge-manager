
CREATE TABLE users (
                       id SERIAL PRIMARY KEY,
                       name VARCHAR(255) NOT NULL,
                       email VARCHAR(255) UNIQUE NOT NULL,
                       position VARCHAR(100),
                       created_at TIMESTAMP DEFAULT NOW(),
                       updated_at TIMESTAMP DEFAULT NOW()
);

-- Insert test data
INSERT INTO users (name, email, position) VALUES
                                              ('John Doe', 'john@example.com', 'Senior Developer'),
                                              ('Jane Smith', 'jane@example.com', 'Project Manager'),
                                              ('Bob Wilson', 'bob@example.com', 'Frontend Developer'),
                                              ('Alice Johnson', 'alice@example.com', 'Backend Developer'),
                                              ('Mike Brown', 'mike@example.com', 'DevOps Engineer');