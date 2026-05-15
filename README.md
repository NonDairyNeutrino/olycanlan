# olycanlan
Discord bot to help manage the Olympia Canadian Highlander League

## Function List
- **Match Reporting.** Intake messages from a channel, parse the match participants, the match record, and match type (bounty, non-bounty first, non-bounty repeated). Assign points to match participants.
  - Ideal scenario is it would write these results to a publically viewable spreadsheet (e.g. google sheet) that can also maintain long term win/loss records of players, deck archetypes, etc.
- **Round Pairings.** Assign pairings every two weeks (every other Monday) based on current league record using typical tournament logic. Post in a channel the pairings (ideally with tags for players). Send reminder in channel a few days before round finish for unreported matches. Add ability to drop a player from the league and re-asdign pairings.
- **Manage League Signups.** Intake signups (no system currently, emotes?) for league. For participants gather/maintain decklists (possible to QA for points??). 
- **General League Announcements.** Including semi-regular point announcement, and standings at the end. Winner(s) announcement. Share decklists once league is finished.
