CREATE TABLE IF NOT EXISTS kyc_verifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    status VARCHAR(20) NOT NULL DEFAULT 'none',
    level INT NOT NULL DEFAULT 0,
    full_name VARCHAR(255) NOT NULL DEFAULT '',
    doc_type VARCHAR(50) NOT NULL DEFAULT '',
    doc_number VARCHAR(100) NOT NULL DEFAULT '',
    doc_image_url TEXT NOT NULL DEFAULT '',
    review_notes TEXT,
    reviewed_by UUID REFERENCES users(id),
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,
    UNIQUE(user_id)
);
