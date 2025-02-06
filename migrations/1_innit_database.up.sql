CREATE TABLE step (
                      id SERIAL PRIMARY KEY,
                      number INT NOT NULL,
                      created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                      updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                      next_step INT NULL,
                      CONSTRAINT fk_step_next_step FOREIGN KEY (next_step) REFERENCES step(id)
);

CREATE TABLE quest (
                       id SERIAL PRIMARY KEY,
                       title VARCHAR NOT NULL,
                       initial_step INT NOT NULL,
                       created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                       updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                       CONSTRAINT fk_quest_initial_step FOREIGN KEY (initial_step) REFERENCES step(id)
);

CREATE TABLE character (
                           id SERIAL PRIMARY KEY,
                           quest INT NOT NULL,
                           name VARCHAR NOT NULL,
                           CONSTRAINT fk_character_quest FOREIGN KEY (quest) REFERENCES quest(id)
);



CREATE TABLE narration_action (
                                  id SERIAL PRIMARY KEY,
                                  step INT NOT NULL,
                                  text VARCHAR NOT NULL,
                                  CONSTRAINT fk_narration_action_step FOREIGN KEY (step) REFERENCES step(id)
);

CREATE TABLE player_action (
                               id SERIAL PRIMARY KEY,
                               step INT NOT NULL,
                               CONSTRAINT fk_player_action_step FOREIGN KEY (step) REFERENCES step(id)
);

CREATE TABLE player_action_choice (
                                      id SERIAL PRIMARY KEY,
                                      player_action INT NOT NULL,
                                      text VARCHAR NOT NULL,
                                      violence_point INT DEFAULT 0,
                                      whatever_point INT DEFAULT 0,
                                      pacifism_point INT DEFAULT 0,
                                      CONSTRAINT fk_player_action_choice FOREIGN KEY (player_action) REFERENCES player_action(id)
);

CREATE TABLE character_action (
                                  id SERIAL PRIMARY KEY,
                                  character INT NOT NULL,
                                  step INT NOT NULL,
                                  CONSTRAINT fk_character_action_character FOREIGN KEY (character) REFERENCES character(id),
                                  CONSTRAINT fk_character_action_step FOREIGN KEY (step) REFERENCES step(id)
);

CREATE TABLE character_action_choice (
                                         id SERIAL PRIMARY KEY,
                                         character_action INT NOT NULL,
                                         text VARCHAR NOT NULL,
                                         violence_point_condition INT DEFAULT 0,
                                         whatever_point_condition INT DEFAULT 0,
                                         pacifism_point_condition INT DEFAULT 0,
                                         CONSTRAINT fk_character_action_choice FOREIGN KEY (character_action) REFERENCES character_action(id)
);

CREATE TABLE playthrough (
                             id SERIAL PRIMARY KEY,
                             player_name TEXT NOT NULL,
                             quest INT NOT NULL,
                             step INT NOT NULL,
                             finished BOOLEAN DEFAULT FALSE,
                             violence_point INT DEFAULT 0,
                             whatever_point INT DEFAULT 0,
                             pacifism_point INT DEFAULT 0,
                             CONSTRAINT fk_playthrough_quest FOREIGN KEY (quest) REFERENCES quest(id),
                             CONSTRAINT fk_playthrough_step FOREIGN KEY (step) REFERENCES step(id)
);
