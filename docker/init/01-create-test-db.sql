-- Banco exclusivo para os testes de integração.
-- Os testes criam e removem somente schemas temporários dentro dele, never
-- tocando no banco de desenvolvimento.
CREATE DATABASE unobot_test;
