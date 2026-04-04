INSERT INTO model_prices (provider, model_name, input_price_per_million, output_price_per_million, effective_from) VALUES
-- OpenAI
('openai', 'gpt-4o',           2.50,   10.00,  '2024-05-13'),
('openai', 'gpt-4o-mini',      0.15,    0.60,  '2024-07-18'),
('openai', 'gpt-4-turbo',     10.00,   30.00,  '2024-04-09'),
('openai', 'gpt-3.5-turbo',    0.50,    1.50,  '2024-01-25'),
('openai', 'o1',              15.00,   60.00,  '2024-12-17'),
('openai', 'o1-mini',          3.00,   12.00,  '2024-09-12'),
('openai', 'o3-mini',          1.10,    4.40,  '2025-01-31'),
-- Anthropic
('anthropic', 'claude-3-5-sonnet',  3.00,   15.00,  '2024-10-22'),
('anthropic', 'claude-3-5-haiku',   0.80,    4.00,  '2024-11-04'),
('anthropic', 'claude-3-opus',     15.00,   75.00,  '2024-02-29'),
('anthropic', 'claude-sonnet-4',    3.00,   15.00,  '2025-05-22'),
('anthropic', 'claude-haiku-4',     0.80,    4.00,  '2025-05-22');
