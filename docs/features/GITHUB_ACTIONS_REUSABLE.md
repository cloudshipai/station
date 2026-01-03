# Station Reusable GitHub Actions

> **Note**: This documentation has been consolidated into the main docs.  
> See [Station GitHub Actions](../station/github-actions.md) for the complete guide.

## Quick Links

- **Run agents in CI/CD**: Use [cloudshipai/station-action](https://github.com/cloudshipai/station-action)
- **Build bundles**: Use `cloudshipai/station/.github/actions/build-bundle`
- **Build images**: Use `cloudshipai/station/.github/actions/build-image`
- **Install CLI**: Use `cloudshipai/station/.github/actions/setup-station`

## Example: Run Agent on PR

```yaml
- uses: cloudshipai/station-action@v1
  with:
    agent: 'Code Reviewer'
    task: 'Review this PR'
  env:
    OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

## Example: Build and Release Bundle

```yaml
- uses: cloudshipai/station/.github/actions/build-bundle@main
  with:
    environment: 'production'
    version: '1.0.0'
```

For complete documentation, see [Station GitHub Actions](../station/github-actions.md).
