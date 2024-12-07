### Step-by-step
- You don't need Golang installed to run this
- You need to open terminal on MacOS or Windows
- You need to then type `./jkwon -query "Something"`
  - Add a "Search Term" in the quotes
  - Hit ENTER
- The scraper should start!

### Usage

The tool supports the following command-line flags:

Required:
- `-query` : Search term for Google Scholar

Optional:
- `-url` : Base URL for Google Scholar (default: https://scholar.google.com/scholar)
- `-start` : Starting page number (default: 0)
- `-lang` : Language for search results (default: en)
- `-sdt` : Scholar document type filter:
  - `0,5` : All documents (default)
  - `0,33` : Articles only
  - `1,5` : Case law only
  - `0` : No patents
  - `2` : Patents only
- `-slow` : Enable slow mode with reduced request rate (default: false)

### Example

`./main -query "My Search Term"`