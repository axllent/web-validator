# Changelog

## [0.1.4]

- Update Go dependencies


## [0.1.3]

- Ignore redirects to CloudFlare (caused by proxied email protection)
- Clean up code
- Update Go dependencies


## [0.1.2]

- Update core modules


## [0.1.1]

- Switch to more reliable [grobotstxt](https://github.com/jimsmart/grobotstxt)


## [0.1.0]

- Display results if process is canceled (ctrl-c)


## [0.0.9]

- Fix validation error counts


## [0.0.8]

- Bugfix for scheme relative links


## [0.0.7]

- Do not show ignored outbound links
- Parse inline CSS & CSS block urls (`url("image.jpg")`)
- Add support for `srcset` links
- Add support for Open Graph images
- Add support for scheme relative links, eg `<script src="//example.com/script.js">`
- Add support for `robots.txt`


## [0.0.6]

- Add `--timeout` configuration
- Add `-t` thread limit (default 5) 


## [0.0.5]

- Add google to ignored links as they almost always redirect to login pages
- Add goroutines & waitgroup for parallel processing


## [0.0.4]

- Add `-i|--ignore` option to skip comma-separated urls
- Add `-r|--redirects` option to report redirects


## [0.0.3]

- Fix report referer formatting
- Rename "external links" to "outbound links"
- Validate command-line URL


## [0.0.2]

- Improve error handling
- Clearer help (`-h`)
- Fix html `base` detection & parsing


## [0.0.1]
- First release
