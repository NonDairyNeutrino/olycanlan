# CLAUDE GENERATED - Probably some weirdness in here. 
# Bot Testing Checklist
# Format: [FILE SET] indicates which JSON files to load before testing
# File sets use shorthand: meta_X = metadata_X, play_X = players_X, seas_X = season_X, mat_X = matches_X

---

## Pre-Testing Setup
- [x] Bot connects to Discord successfully
- [x] All JSON files load without error on startup
- [x] All slash commands register in the guild
- [x] .env contains all required variables

---

## /signup battler
# [meta_A + play_A + seas_A + mat_A] — Fresh league, no prior data
- [x] User with no roles signs up — receives Battler role
- [x] User with no roles signs up — season.json creates fresh player entry with correct defaults
- [x] User with no roles signs up — players.json creates fresh historical entry
- [x] User with no roles signs up — receives ephemeral confirmation
- [x] User signs up — nickname used if set, username used if no nickname

# [meta_A + play_B + seas_B + mat_A] — Returning player (exists in both JSONs)
- [x] Existing player signs up — data updated, not duplicated in season.json
- [x] Existing player signs up — nickname updated in players.json, not duplicated

# [meta_A + play_B + seas_B + mat_A] — Role conflict
- [x] User already a Battler signs up — receives "already signed up" ephemeral, no data changed
- [x] User who is a Jammer signs up as Battler — Jammer role removed, Battler role added, season.json role updated

# [meta_B + play_B + seas_B + mat_A] — Signups closed
- [x] Signups closed — user receives "signups closed" ephemeral, no data changed

---

## /signup jammer
# [meta_A + play_A + seas_A + mat_A]
- [x] User with no roles signs up — receives Jammer role
- [x] User signs up — season.json creates fresh entry with role "jammer"
- [x] User signs up — players.json creates fresh historical entry
- [x] User signs up — receives ephemeral confirmation
- [x] User signs up — nickname used if set, username if not

# [meta_A + play_B + seas_B + mat_A]
- [x] Existing player signs up — data updated not duplicated
- [x] User already a Jammer — receives "already signed up" ephemeral
- [x] User who is a Battler signs up as Jammer — Battler role removed, Jammer role added

---

## /signup decklist
# [meta_A + play_B + seas_B + mat_A] — Normal submission
- [x] Non-Battler runs command — receives "Battlers only" ephemeral
- [x] Valid URL submitted — season.json decklist updated with URL and name
- [x] Valid URL submitted without https:// — URL normalized correctly
- [⚠️] Invalid URL submitted — receives "invalid URL" ephemeral, no data changed
- [x] Valid submission — admin channel receives decklist review embed with Approve/Reject buttons
- [x] Valid submission — embed footer contains correct player ID
- [x] Valid submission — user receives confirmation ephemeral
- [x] Deck name omitted — submission succeeds with empty name
- [x] Player resubmits decklist — old decklist overwritten

# [meta_A + play_B + seas_B + mat_A] — Missing season data edge case
- [x] Battler with Battler role but NO season.json entry — receives error ephemeral, admin channel notified

# [meta_B + play_B + seas_B + mat_A] — Signups closed
- [x] Signups closed, no decklist submitted — receives "signups closed, no decklist" ephemeral
- [x] Signups closed, decklist previously submitted — receives "use original" ephemeral with URL

---

## Decklist Review Buttons (approved / rejected)
# Requires a real decklist review message in the admin channel from a /signup decklist submission
- [x] Approve clicked — season.json decklist approved set to true
- [x] Approve clicked — player receives DM confirmation
- [x] Approve clicked — embed title changes to "Decklist Approved ✅", color green
- [x] Approve clicked — buttons removed from message
- [x] Reject clicked — player receives DM denial
- [x] Reject clicked — embed title changes to "Decklist Denied ❌", color red
- [x] Reject clicked — buttons removed from message
- [x] Reject clicked — season.json approved remains false
- [ ] Button on non-bot message — silently ignored
- [ ] Button on wrong embed title — silently ignored
- [ ] Button on embed with no footer — silently ignored

---

## /drop
# [meta_C + play_B + seas_C + mat_B] — Active player drops
- [ ] Non-participant runs command — receives "not active" ephemeral
- [x] Battler drops — Battler role removed, Inactive role added
- [x] Battler drops — season.json dropped: true, active: false
- [x] Battler drops — receives ephemeral confirmation
- [x] Battler drops — admin channel notified with player and role
- [x] Jammer drops — Jammer role removed, Inactive role added, season.json updated
- [x] Drop with reason — reason in admin channel message
- [x] Drop with no reason — "No Reason Provided" in admin message
- [ ] Verify no deadlock after drop (subsequent commands work normally) [defer bug fix check]

---

## /league open-signups
# [meta_B + play_B + seas_B + mat_A] — Signups already closed
- [x] Non-organizer runs command — receives "admins only" ephemeral
- [x] Organizer opens when closed — metadata signups set to true
- [x] Organizer opens when closed — receives confirmation ephemeral

# [meta_A + play_B + seas_B + mat_A] — Signups already open
- [x] Organizer opens when already open — receives "already open" ephemeral

---

## /league close-signups
# [meta_A + play_B + seas_B + mat_A] — Signups open
- [x] Non-organizer runs command — receives "admins only" ephemeral
- [x] Organizer closes — metadata signups set to false
- [x] Organizer closes — active_players count correct
- [x] Organizer closes — battlers count correct (verify typo fix: "battler" not "battlers")
- [x] Organizer closes — total_rounds = ceil(log2(battlers)) correct
- [x] Organizer closes — receives confirmation ephemeral

# [meta_B + play_B + seas_B + mat_A] — Signups already closed
- [x] Organizer closes when already closed — receives "already closed" ephemeral

# Edge case: manually set battlers to 0 in metadata_A before closing
- [x] 0 battlers — verify log2(0) handled gracefully, no panic

---

## /league new-season
# [meta_A + play_B + seas_C + mat_B] — Valid new season start (signups currently closed)
- [x] Non-organizer runs command — receives "admins only" ephemeral
- [x] Valid date — metadata season number incremented
- [x] Valid date — metadata signups set to true
- [x] Valid date — metadata start_date updated
- [x] Valid date — metadata current_round reset to 0
- [x] Valid date — current matches archived with status "archived"
- [x] Valid date — current_season matches cleared
- [x] Valid date — next_match_id reset to 1
- [x] Valid date — each player historical record updated from season standings
- [x] Valid date — last_decklist updated for battlers only
- [x] Valid date — seasons_played updated for each player
- [x] Valid date — old season.json archived to site/data/archive/season-X.json
- [x] Valid date — season.json reset to empty rounds and season_players
- [x] Valid date — all JSON files saved
- [x] Valid date — receives confirmation ephemeral
- [x] Valid date — season opening announcement posted with @everyone ping

# [meta_A + play_B + seas_C + mat_B] — Invalid inputs
- [x] Invalid date format — receives date format error, no data changed
- [x] Signups currently open — receives "league already open" ephemeral

# Edge case: player in season.json but NOT in players.json
- [ ] Manually remove one player from players_B before loading — verify graceful handling, no panic

---

## /round new
# [meta_B + play_B + seas_B + mat_A] — First round (current_round == 0)
- [ ] Non-organizer runs command — receives "admins only" ephemeral
- [ ] current_round == 0 — no previous round completion check (verify fix, no panic)
- [ ] Even player count (16 battlers) — no bye generated
- [ ] Pairings generated — all within same win bracket where possible
- [ ] No rematches in generated pairings
- [ ] Valid pairings saved to rounds.pending in season.json
- [ ] Admin receives ephemeral embed with all pairings and records

# [meta_B + play_B + seas_B + mat_A] — Odd player count
- [ ] Manually set one battler to active: false to make count odd — bye generated
- [ ] Bye assigned to player without prior bye
- [ ] Player who received bye (received_bye: true) is not assigned bye again

# [meta_E + play_B + seas_E + mat_B] — Season complete
- [ ] current_round == total_rounds — receives "no additional round needed" ephemeral

# [meta_C + play_B + seas_C + mat_B] — Previous round not completed
- [ ] Previous round status not "completed" — receives "complete previous round first" ephemeral

# [meta_D + play_B + seas_E + mat_B] — Lone undefeated player
- [ ] Only one player at max wins — receives tournament over ephemeral with winner mention

# [meta_B + play_B + seas_B + mat_A] — Generation failure
- [ ] Manually construct exhausted pairing data where no valid pairing possible — fails after 50 attempts, admin notified

---

## /round post
# [meta_B + play_B + seas_B + mat_A] — After /round new has been run (pending exists)
- [ ] Non-organizer runs command — receives "admins only" ephemeral
- [ ] Pending round exists — current_round incremented in metadata
- [ ] Pending round exists — round moved from pending to numbered key
- [ ] Pending round exists — status set to "active"
- [ ] Pending round exists — pending set to nil
- [ ] Pending round exists — season.json saved
- [ ] Pairings embed posted to MATCHES_CHNL_ID with correct round number
- [ ] Each pairing shows correct player mentions and records
- [ ] Bye shown correctly in embed
- [ ] Announcement pings @everyone
- [ ] Admin receives ephemeral confirmation with channel link

# [meta_B + play_B + seas_B + mat_A] — No pending round
- [ ] No pending round (pending is nil/empty) — receives "no pending round" ephemeral

---

## /round check
# [meta_C + play_B + seas_D + mat_D] — Partial reporting (4 unreported)
- [ ] Non-organizer runs command — receives "admins only" ephemeral
- [ ] Unreported matches — receives ephemeral listing unreported matches by table
- [ ] Unreported matches — Post Reminder button present

# Post Reminder button from /round check
- [ ] Button clicked — reminder posted to MATCHES_CHNL_ID
- [ ] Button clicked — embed title updated to "Unreported Bounty Reminder"
- [ ] Button clicked — admin receives confirmation ephemeral

# [meta_C + play_B + seas_D + mat_E] — All reported
- [ ] All matches reported — receives "all reported" ephemeral

# [meta_B + play_B + seas_B + mat_A] — Round not active
- [ ] Round status not "active" — receives "round not active" ephemeral

---

## /round close
# [meta_C + play_B + seas_D + mat_C] — None reported
- [ ] Non-organizer runs command — receives "admins only" ephemeral
- [ ] All round 3 matches unreported — receives unreported bounties embed
- [ ] Unreported embed — Post Reminder button present
- [ ] Unreported embed — no data modified, no save triggered

# [meta_C + play_B + seas_D + mat_D] — Partial reporting
- [ ] 4 unreported matches — embed lists exactly those 4 tables

# [meta_C + play_B + seas_D + mat_E] — All reported, ready to close
- [ ] All reported — pairing data updated: match_id, winner, result, reported: true
- [ ] Bounty match winner gets 3 points
- [ ] Bounty match loser gets 1 point
- [ ] Non-bounty match winner gets 1 point, loser gets 0
- [ ] First time opponents — both get +3 points
- [ ] First time opponents — both added to each other's opponents list
- [ ] Repeat opponents — no +3 bonus, opponents list unchanged
- [ ] Winner wins += 1
- [ ] Loser losses += 1
- [ ] Winner game_wins and game_losses updated correctly from result string
- [ ] Loser game_wins and game_losses updated correctly (reversed)
- [ ] Concession result (0-0) — game wins/losses both 0
- [ ] Bye player — wins += 1, points += 3, received_bye = true
- [ ] Match status set to "logged" for all processed matches
- [ ] Round status set to "completed"
- [ ] Player pairings list updated with new opponent (verify fix)
- [ ] All data saved before unlock
- [ ] Standings embed posted to SEASON_CHNL_ID grouped by record
- [ ] Within each record group, players sorted by points descending
- [ ] Jammers excluded from standings embed
- [ ] Admin receives ephemeral confirmation

# [meta_B + play_B + seas_B + mat_A] — Round not active
- [ ] Round not active — receives "round not active" ephemeral

---

## /result
# [meta_C + play_B + seas_D + mat_D] — Active round, normal submission
- [ ] Non-participant runs command — receives "league members only" ephemeral
- [ ] Round not active — receives "no active round" ephemeral
- [ ] Non-bounty match — saved to matches.json with correct data and status "active"
- [ ] Non-bounty match — next_match_id incremented
- [ ] Non-bounty match — announcement embed posted to BOUNTY_CHNL_ID
- [ ] Non-bounty match — msg_id saved back to matches.json
- [ ] Non-bounty match — user receives ephemeral with link to announcement
- [ ] Match ID format correct (e.g. S08-021)

# Bounty match checks
- [ ] Bounty submitted with no matching pairing in current round — receives "no bounty found" ephemeral
- [ ] Bounty submitted, valid pairing exists — proceeds to duplicate check
- [ ] Bounty already reported for this pairing (S08-017 exists in mat_D) — receives ephemeral with link
- [ ] Bounty not yet reported — saved successfully
- [ ] Winner/loser flipped vs pairing order — still identified as bounty correctly

# Edge cases
- [ ] Concession result (0-0) submitted — saved correctly
- [ ] Winner == Loser submitted — verify behavior (Discord should prevent via user picker but worth noting)

---

## /admin player-signup
# [meta_A + play_B + seas_B + mat_A]
- [ ] Non-organizer runs command — receives "admins only" ephemeral
- [ ] Battler signup, signups closed — receives "signups closed" ephemeral
- [ ] Battler signup, player already Battler — receives "already signed up" ephemeral
- [ ] Battler signup, player is Jammer — Jammer role removed, Battler role added
- [ ] Battler signup, new player — season.json and players.json entries created correctly
- [ ] Battler signup, existing player — data updated not duplicated
- [ ] Battler signup — admin receives confirmation ephemeral
- [ ] Jammer signup, player already Jammer — receives "already signed up" ephemeral
- [ ] Jammer signup, player is Battler — Battler role removed, Jammer role added
- [ ] Jammer signup, new player — entries created correctly
- [ ] Jammer signup — admin receives confirmation ephemeral

---

## /admin player-drop
# [meta_C + play_B + seas_D + mat_D]
- [ ] Non-organizer runs command — receives "admins only" ephemeral
- [ ] Player not active participant — receives "not active" ephemeral
- [ ] Battler dropped — Battler role removed, Inactive role added
- [ ] Battler dropped — season.json dropped: true, active: false
- [ ] Battler dropped — admin receives confirmation ephemeral
- [ ] Battler dropped — admin channel notified
- [ ] Jammer dropped — Jammer role removed, Inactive role added, season.json updated
- [ ] Verify no deadlock after drop (defer bug fix check — subsequent commands work)

---

## /admin player-points
# [meta_C + play_B + seas_D + mat_D]
- [ ] Non-organizer runs command — receives "admins only" ephemeral
- [ ] Player not in season data — receives "no data found" ephemeral
- [ ] Add 10 points — points increased by exactly 10
- [ ] Subtract 5 points — points decreased by exactly 5
- [ ] Subtract more than current points — points floored at 0
- [ ] Set to 25 — points set to exactly 25
- [ ] Set to -5 — points floored at 0
- [ ] Points saved to season.json correctly
- [ ] Admin receives confirmation ephemeral with old and new values
- [ ] Admin channel receives adjustment notification with all details

---

## /admin player-info
# [meta_C + play_B + seas_D + mat_D] — Full data player
- [ ] Non-organizer runs command — receives "admins only" ephemeral
- [ ] Battler with full data — all fields populated correctly
- [ ] Battler — decklist shown as linked name/URL
- [ ] Jammer — decklist shown as N/A
- [ ] Opponents list formatted as mentions
- [ ] Seasons played list formatted correctly (e.g. S6, S7)
- [ ] Embed received as ephemeral
- [ ] Typo check: "HISTORICAL DATA" not "HISORICAL DATA"

# [meta_C + play_A + seas_A + mat_A] — Player with no data
- [ ] Player with no season data — season section shows "no data" message
- [ ] Player with no historical data — historical section shows "no data" message

# Edge cases
- [ ] Player with empty opponents list — no panic
- [ ] Player with empty seasons_played — no panic

---

## /admin match-edit
# [meta_C + play_B + seas_D + mat_D] — Active matches available (S08-017 through S08-020)
- [ ] Non-organizer runs command — receives "admins only" ephemeral
- [ ] Invalid match ID — receives "not found" ephemeral
- [ ] Match already logged (S08-001) — receives "already logged" ephemeral
- [ ] Match already voided (S08-020 in mat_B) — receives "already voided" ephemeral
- [ ] No fields changed — receives "no changes detected" ephemeral
- [ ] Winner == Loser submitted — receives "cannot match" ephemeral
- [ ] Winner changed only — loser unchanged, data updated
- [ ] Loser changed only — winner unchanged, data updated
- [ ] Result changed only — other fields unchanged
- [ ] Bounty changed only — other fields unchanged
- [ ] Multiple fields changed simultaneously — all updated correctly
- [ ] Match status set to "edited" after change
- [ ] Original announcement embed updated in BOUNTY_CHNL_ID
- [ ] Admin receives confirmation ephemeral with old/new values and message link
- [ ] matches.json saved correctly

---

## /admin match-delete
# [meta_C + play_B + seas_D + mat_D] — Active matches available
- [ ] Non-organizer runs command — receives "admins only" ephemeral
- [ ] Invalid match ID — receives "not found" ephemeral
- [ ] Match already logged (S08-001) — receives "already logged" ephemeral
- [ ] Valid active match — status set to "voided" in matches.json
- [ ] Valid match — original embed edited to "Match Voided" in BOUNTY_CHNL_ID
- [ ] Valid match — admin receives confirmation ephemeral with message link
- [ ] matches.json saved correctly

---

## Concurrency & Data Integrity
# Run these with the bot under load using two Discord accounts simultaneously
- [ ] Two users submit /result simultaneously — no data corruption, both matches saved
- [ ] Admin runs /round close while player submits /result — no deadlock or panic
- [ ] Bot shutdown mid-command (Ctrl+C) — data saved at shutdown is consistent
- [ ] Bot restarted after any operation — JSON loads correctly, bot resumes normally

---

## Full End-to-End Season Flow
# Start fresh: [meta_A + play_A + seas_A + mat_A]
- [ ] /league open-signups — signups open
- [ ] Multiple players /signup battler — all receive roles and data created
- [ ] Multiple players /signup jammer — all receive roles and data created
- [ ] Players /signup decklist — admin reviews and approves all
- [ ] /league close-signups — total_rounds calculated correctly
- [ ] /round new (round 1) — pairings generated, no panics on round 0
- [ ] /round post — pairings announced in matches channel
- [ ] /result submitted for all pairings (bounty and non-bounty)
- [ ] /round check — confirms all reported
- [ ] /round close — points distributed correctly, standings posted
- [ ] Repeat /round new through /round close for all rounds
- [ ] Final /round new — lone undefeated player detected, tournament over message sent
- [ ] /league new-season — all data archived, fresh season initialized

---

## ADDED AFTER BOT
- [ ] /league new-season clears battler/jammer roles and adds past league player role
