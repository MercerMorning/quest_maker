
-- Уникальный индекс для предотвращения дублирования выборов одного игрока на одном шаге
ALTER TABLE player_choice 
ADD CONSTRAINT unique_player_choice_per_step 
UNIQUE (multiplayer_playthrough, player_name, step_id);
