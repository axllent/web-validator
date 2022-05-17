# Validate website HTML & CSS, check links & resources

[![Go Report Card](https://goreportcard.com/badge/github.com/axllent/web-validator)](https://goreportcard.com/report/github.com/axllent/web-validator)

A command-line website validator for Linux, Mac & Windows, which can spider through a website, 
validating all HTML & CSS pages, check the existence of all assets (images, css, fonts etc), 
and verify outbound links.


## Features

- Check a single URL, to a certain depth, or an entire website
- HTML & CSS validation using (default) the [Nu Html Checker](https://validator.w3.org/)
- Detect & checks linked assets from HTML & linked CSS (fonts, favicons, images, videos, etc)
- Detect mixed content (HTTPS => HTTP) for linked assets (fonts, images, CSS, JS etc)
- Verify outbound links (to external websites)
- Summary report or errors (& optionally HTML/CSS warnings)
- Obeys `robots.txt` (can be ignored)


## Usage options

```shell
Usage: web-validator [options] <url>

Options:
  -a, --all                recursive, follow all internal links (default single URL)
  -d, --depth int          crawl depth ("-a" will override this)
  -o, --outbound           check outbound links (HEAD only)
      --html               validate HTML
      --css                validate CSS
  -i, --ignore string      ignore URLs, comma-separated, wildcards allowed (*.jpg,example.com)
  -n, --no-robots          ignore robots.txt (if exists)
  -r, --redirects          treat redirects as errors
  -w, --warnings           display validation warnings (default errors only)
  -f, --full               full scan (same as "-a -r -o --html --css")
  -t, --threads int        number of threads (default 5)
      --timeout int        timeout in seconds (default 10)
      --validator string   Nu Html validator (default "https://validator.w3.org/nu/")
  -u, --update             update to latest release
  -v, --version            show app version
```


## Examples

- `web-validator https://example.com/` - scan URL, verify all direct assets & links
- `web-validator https://example.com/ --css --html` - scan URL, verify all direct assets & links, validate HTML & CSS
- `web-validator https://example.com/ -a` - scan entire site, verify assets & links
- `web-validator https://example.com/ --css --html -d 2` - scan site to a depth of 2 internal links, verify assets & links, validate HTML and CSS
- `web-validator https://example.com/ -a -o` - scan entire site, verify all assets, verify outbound links
- `web-validator https://example.com/ -f` - scan entire site, verify all assets, verify outbound links, validate HTML & CSS


## Installing

Download the [latest binary release](https://github.com/axllent/web-validator/releases/latest) for your system, 
or build from source `go get -u github.com/axllent/web-validator`(go >= 1.11 required)


## FAQ

### When I scan a single page, web-validator scans many other pages too

When scanning a page, the software will check all internal links from that single page, which include both pages and files. Only a HEAD request is done on linked pages (no validation etc) to check for a valid response.


### Web-validator says some of my outbound links are broken, however they do work

Some sites specifically block all HEAD requests, in which case web-validator will try a regular GET request. Some sites however go to extreme lengths to prevent any kind of scraping, such as LinkedIn, so these will always return an error response. LinkedIn (specifically) is now blacklisted in the application, so any linkedin links are completely ignored. If you come across another major site with similar issues, then let me know and I will add them to the list.


### HTML/CSS validation

Validation uses the [Nu Html validator]("https://validator.w3.org/nu/"), and by default uses the online public service (they [encourage this](https://github.com/validator/validator/wiki/Service-%C2%BB-Input-%C2%BB-POST-body)). You can however use your [own instance](https://validator.w3.org/docs/users.html) of the validator (open source), and use the `--validator <your-server>` to specify your own.


### Robots.txt

By default, web-validator obeys `Disallow` rules in `robots.txt` if it exists. You can optionally skip this by adding `-n` to your runtime flags. To add specific rules for just the validator, you can target it specifically with `User-agent: web-validator`, eg:

```
User-agent: web-validator
Disallow: /assets/Products/*
```
