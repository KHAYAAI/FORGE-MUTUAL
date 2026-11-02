-- One database per service that keeps state. Services migrate their own schema when they start.
-- Temporal creates its own databases (temporal, temporal_visibility) through its auto-setup image.
create database party;
create database consent;
create database onboarding;
create database payments;
create database payouts;
create database fx;
create database consolidated;
create database recon;
create database treasury;
create database cases;
create database audit;
create database policy;
create database notifications;
create database partner_router;
create database ledger;
