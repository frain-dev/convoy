-- +migrate Up
create unlogged table if not exists convoy.circuit_breaker (
    key text not null primary key,
    state text not null default 'closed',

    -- successes is used for transitioning out of the half-open state
    successes integer not null default 0,

    -- this is the last_error received before reseting 
    -- successes and transitioning back to the open state
    last_error text,
	
    -- this is used to determine how we transition to half-open
    circuit_opened_at timestamptz,

    created_at timestamptz default now(),
    updated_at timestamptz default now()
);

-- +migrate Down
drop table if exists convoy.circuit_breaker;
