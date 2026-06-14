# olycanlan
Discord bot to help manage the Olympia Canadian Highlander League

## Function List
- **Match Reporting.** Intake messages from a channel, parse the match participants, the match record, and match type (bounty, non-bounty first, non-bounty repeated). Assign points to match participants.
  - Ideal scenario is it would write these results to a publically viewable spreadsheet (e.g. google sheet) that can also maintain long term win/loss records of players, deck archetypes, etc.
- **Round Pairings.** Assign pairings every two weeks (every other Monday) based on current league record using typical tournament logic. Post in a channel the pairings (ideally with tags for players). Send reminder in channel a few days before round finish for unreported matches. Add ability to drop a player from the league and re-asdign pairings.
- **Manage League Signups.** Intake signups (no system currently, emotes?) for league. For participants gather/maintain decklists (possible to QA for points??). 
- **General League Announcements.** Including semi-regular point announcement, and standings at the end. Winner(s) announcement. Share decklists once league is finished.

## Slash Commands Heirarchy
Finished 🟩 | Developed 🟨 | Nonstarted 🟥

### **PLAYER COMMANDS**


#### `/result` 🟨

**Required Roles:** Battler & Jammer  
**Description:**  Records a completed match result. Collects winner, loser, record, type (bounty/non-bounty) and generates a matchID. Adds match to matches data. NEEDS to write some data to pairings (reported, matchID, winner, etc)

---

#### `/signup battler` 🟩

**Description:**  Registers the command giver as a Battler for the current season and assigns the correct role. Usable only while signups are OPEN.

---

#### `/signup jammer` 🟩

**Description:**  Registers the command giver as a Jammer for the current season and assigns the correct role.

---

#### `/signup decklist` 🟩

**Required Roles:** Battler  
**Description:**  Stores or updates the Battler's submitted decklist for the season. Pings organizer to review for point spread.

---

#### `/drop` 🟩

**Required Roles:** Battler & Jammer
**Description:**  Removes a player from the current season. Indicates in their player data that they are inactive for future rounds.

---

### **ADMIN COMMANDS**

#### `/league open-signups` 🟩
**Description:**  Opens league registration, enabling player signups and updating system state to allow role assignment and enrollment actions.

---

#### `/league close-signups` 🟩
**Description:**  Closes league registration, disabling player signups and updating system state to allow role assignment and enrollment actions.

---

#### `/league new-season` 🟩
**Description:**  Initializes a new season. Clears current season match history, resets player roles, makes new season announcement.

---

#### `/round new` 🟨
**Description:**  Generates pairings for the next round using current standings, assigns matchups and byes, and writes round structure to season data. 

---

#### `/round post` 🟨
**Description:**  Publishes round and matches info in matchups channel. Separated from `/round new` to allow for QA.

---

#### `/round close` 🟨
**Description:**  Closes the currently active round. How does it handle un-reported matches?

---

#### `/round reminder` 🟨
**Description:**  Publishes a reminder for all unreported/incomplete matches in the active round

---

#### `/admin match-edit` 🟩
**Description:**  Allows revision of a match's data using its matchID. Potential to edit original message (NEED TO ADD MSGID to /RESULT). Changes status to "edited"

---

#### `/admin match-delete` 🟩
**Description:**  Removes a match from the matches data using its matchID. Changes status to "voided"

---

#### `/admin player-signup` 🟩
**Description:**  Admin verion of the signup command. Allows selection of battler/jammer

---

#### `/admin player-drop` 🟩
**Description:**  Drops a player from the current league season

---

#### `/admin player-points` 🟩
**Description:**  Manually adjusts league points to a player

---

#### `/admin player-info` 🟩
**Description:**  Replies with a players role, current record, decklist, match history?
