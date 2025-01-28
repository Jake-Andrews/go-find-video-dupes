# govdupes
Uses perceptual hashing of screenshots taken from videos to detect duplicate
but not identical (different encoding, slight filter, short duration differences) videos. 

## Screenshots

Group size = Sum of directory entries with unique inode & device id
- ![Display Duplicates](static/display_duplicates.png)
- ![Searching Screen](static/searching.png)
- ![Settings Screen](static/settings.png)
Hardlink will hardlink each selected video in the video group's row to the
first video in said row. 
- ![Select and Delete Hardlink](static/select_delete_hardlink.png)
- ![Statistics Screen](static/statistics.png)
- ![Search Screen](static/search.png)

