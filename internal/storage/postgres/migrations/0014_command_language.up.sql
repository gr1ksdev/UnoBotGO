-- Cada grupo escolhe o idioma dos seus comandos. Não há aliases cruzados:
-- um grupo em português aceita apenas /novo, /entrar...; um grupo em inglês
-- aceita apenas /new, /join...
ALTER TABLE group_configs ADD COLUMN command_language text NOT NULL DEFAULT 'pt'
  CHECK (command_language IN ('pt', 'en'));
