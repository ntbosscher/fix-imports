
# Fix-Imports

Refactoring a big JS library and breaking imports? 

This is a fast way to automatically update your incorrect imports after a refactor.

```bash
import {Header} from "../Misc/Header";
-> Error: Cannot find module "../Misc/Header"

> fix-imports --write
> scanning project/src...
> "../Misc/Header" -> import {Header} from "../../Header";
```

## Algorithm

By default fix-imports runs with `--write=false`, so you can see what updates will happen before applying them.

- Scan all imports (project with 800 files takes about 1.2s on my MBP)
- For imports that don't resolve, search for potential matches (e.g. `../Misc/Header`)
  - Search for `Misc/Header.ts` (and similar)
  - If one possible match is found, use that one
  - If multiple matches are found, choose the one that is the closest in the directory structure
  - If none are found, remove folders and run search again `Header.ts`
  
All changes are logged to the console, so you can see what's going on.