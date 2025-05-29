
-- 0 = для всех игроков, 1 = для первого игрока, 2 = для второго игрока
ALTER TABLE player_action_choice 
ADD COLUMN player_index INT DEFAULT 0;
