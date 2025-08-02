# next-dev-eager

exactly what it sounds like. frustrated by slow page load times in next.js after starting up your local dev server?

assuming you've already run `npm run dev`,

just run `npx next-dev-eager`. 

that's it. we'll warm up all the routes for you.

## contributing

to generate a binary in `./bin/` and try out the program,

```bash
make # Build a binary for UNIX
```

to make a release,

```
make build-all
make version-patch|minor|major
make publish
```

## roadmap

- [ ] accept custom `app` location as an arg
- [ ] enable ability to pipe `next dev` output INTO `next-dev-eager`, unlocking...
    - [ ] live re-prioritization of which route to warm-up next