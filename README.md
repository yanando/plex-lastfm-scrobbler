# Improved Last.FM scrobbler for Plex

## Usage
```lastfm_scrobbler -t YOUR_PLEX_TOKEN```

Pass the ``-h`` flag to view all possible command line arguments.
### You can optionally provide a config file
```lastfm_scrobbler -c ./config.yaml```
The following variables can be defined in your config file:
```yaml
server url: http://localhost:32400
plex token: xxxxx
lastfm user: userx
plex music library: Music
verbose: true
```
