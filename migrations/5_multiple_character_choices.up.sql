
ALTER TABLE character_action_choice ADD COLUMN next_step INT NULL;
ALTER TABLE character_action_choice ADD CONSTRAINT fk_character_action_choice_next_step 
    FOREIGN KEY (next_step) REFERENCES step (id);

ALTER TABLE character_action_choice ADD COLUMN priority INT DEFAULT 0;
