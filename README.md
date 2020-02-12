# Web-validator - Validate website HTML & CSS, check links & resources

A command-line website validator for Linux, Mac & Windows, which can spider through a website, 
validating all HTML & CSS pages, check the existence of all assets (images, css, fonts etc), 
and verify external links.


## Features

- Check a single URL, to a certain depth, or an entire website
- HTML & CSS validation using (default) the [Nu Html Checker](https://validator.w3.org/)
- Detect & checks linked assets from HTML & CSS (CSS, favicons, images, videos, etc)
- Detect protocol downgrading (HTTPS => HTTP) for linked assets (images, CSS, JS etc)
- Verify links to external sites
- Summary report or errors


## Usage options

```shell
Usage: web-validator [options] <url>

Options:
  -a, --all                recursive, follow all internal links (default single URL)
  -d, --depth int          crawl depth ("-a" will override this)
  -e, --external           check external links (HEAD only)
      --html               validate HTML
      --css                validate CSS
  -w, --warnings           display warnings too (default only show errors)
  -f, --full               full scan (same as "-a -e --html --css")
      --validator string   Nu Html validator (default "https://validator.w3.org/nu/")
  -u, --update             update to latest release
  -v, --version            show app version
```

## Examples

- `web-validator https://example.com/` - scan just this URL, verify all direct assets & links
- `web-validator https://example.com/ --css --html` - scan URL, verify all direct assets & links, validate HTML & CSS
- `web-validator https://example.com/ -a` - scan entire site, verify assets & links
- `web-validator https://example.com/ --css --html -d 2` - scan site to a depth of 2 internal links, verify assets & links, validate HTML and CSS
- `web-validator https://example.com/ -e` - scan entire site, verify all assets, verify external links
- `web-validator https://example.com/ -f` - scan entire site, verify all assets, verify external links, validate HTML & CSS


## Installing

Download the [latest binary release](https://github.com/axllent/web-validator/releases/latest) for your system, 
or build from source `go get -u github.com/axllent/web-validator`(go >= 1.11 required)
