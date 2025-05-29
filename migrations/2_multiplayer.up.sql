
CREATE TABLE game_server
(
    id          SERIAL PRIMARY KEY,
    quest_id    INT     NOT NULL,
    server_name VARCHAR NOT NULL,
    is_public   BOOLEAN DEFAULT TRUE,
    max_players INT     DEFAULT 2,
    status      VARCHAR DEFAULT 'waiting', -- waiting, in_progress, finished
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_game_server_quest FOREIGN KEY (quest_id) REFERENCES quest (id)
);

CREATE TABLE server_player
(
    id          SERIAL PRIMARY KEY,
    server_id   INT  NOT NULL,
    player_name TEXT NOT NULL,
    joined_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_server_player_server FOREIGN KEY (server_id) REFERENCES game_server (id)
);

CREATE TABLE multiplayer_playthrough
(
    id             SERIAL PRIMARY KEY,
    server_id      INT NOT NULL,
    current_step   INT NOT NULL,
    violence_point INT DEFAULT 0,
    whatever_point INT DEFAULT 0,
    pacifism_point INT DEFAULT 0,
    status         VARCHAR DEFAULT 'active',
    CONSTRAINT fk_multiplayer_playthrough_server FOREIGN KEY (server_id) REFERENCES game_server (id),
    CONSTRAINT fk_multiplayer_playthrough_step FOREIGN KEY (current_step) REFERENCES step (id)
);

CREATE TABLE player_choice
(
    id                      SERIAL PRIMARY KEY,
    multiplayer_playthrough INT  NOT NULL,
    player_name             TEXT NOT NULL,
    choice_id               INT  NOT NULL,
    step_id                 INT  NOT NULL,
    created_at              TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_player_choice_playthrough FOREIGN KEY (multiplayer_playthrough) REFERENCES multiplayer_playthrough (id),
    CONSTRAINT fk_player_choice_step FOREIGN KEY (step_id) REFERENCES step (id)
);
