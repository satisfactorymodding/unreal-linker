# unreal-linker
Small utility to allow us to have our UE repo in our organisation while respecting Epic's restrictions on who can access UE source code

## Setup
The Docker image is available at `ghcr.io/satisfactorymodding/unreal-linker:main`.

See this sample Docker Compose for variables to set:
```yaml
version: '3.8'

services:
  unreal-linker:
    restart: unless-stopped
    image: ghcr.io/satisfactorymodding/unreal-linker:main
    ports:
      - 8080:8080
    environment:
      - GITHUB_OAUTH_ID=po9gepmd8jskg6cfw2js
      - GITHUB_OAUTH_SECRET=vrpiv4brcjt8a57pcutop63pbzwi877hp4jg58pu9
      - GITHUB_APP_ID=654321
      - GITHUB_INSTALLATION_ID=12345678
      - GITHUB_APP_KEY_PATH=path/to/your/unreal-linker.private-key.pem
      - GITHUB_REPOSITORY=SatisfactoryModding/UnrealEngine
```
*Disclaimer:* Keep all your OAuth and App keys/secrets secure, as they have powerful permissions.
Review GitHub's instructions and guidelines for securing OAuth Apps and GitHub Apps.

### OAuth App
The OAuth app is used to make requests on behalf of the user, who is trying to link their account.

1. Create a new OAuth app in your GitHub organization settings.
2. The Authorization Callback URL must be a publicly reachable link to the `/authorize` endpoint,
   for example `https://yourserver.com/authorize`.
3. Copy the Client ID to `GITHUB_OAUTH_ID`.
4. Generate a new client secret and copy it to `GITHUB_OAUTH_SECRET`.

### GitHub App
The GitHub app is used to make requests on behalf of your organization, to invite the user to the repository.

1. Create a new GitHub App in your organization settings. Most of the details are unimportant, as this is not user-facing.
    - Webhooks can be disabled
    - Under "Repository Permissions" section set "Administration" to Read & Write
    - Under "Where can this GitHub App be installed?" choose "Only on this account"
2. After creating the app, copy the App ID to `GITHUB_APP_ID`.
3. Scroll down to "Private keys" and generate a new one. The path to this key on your server is `GITHUB_APP_KEY_PATH`
4. Navigate to the "Install App" section and install it to your organization.
5. Go to the installed App in your organization settings. The last part of the URL should have the `GITHUB_INSTALLATION_ID`.
   For example `https://github.com/organizations/SatisfactoryModding/settings/installations/12345678`.
