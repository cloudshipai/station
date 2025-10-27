# CloudShip AI Station Action - Deployment Guide

This guide walks through creating and publishing the CloudShip AI Station GitHub Action.

## Prerequisites

- GitHub account with ability to create public repositories
- Access to `ghcr.io/cloudshipai/station-security:latest` Docker image (already built and public)
- GitHub organization: `cloudshipai`

## Step-by-Step Deployment

### Step 1: Create Public Repository

```bash
# Using GitHub CLI
gh repo create cloudshipai/station-action \
  --public \
  --description "AI-powered security scanning for GitHub Actions" \
  --homepage "https://cloudshipai.com"

# Or via web:
# 1. Go to https://github.com/organizations/cloudshipai/repositories/new
# 2. Name: station-action
# 3. Description: AI-powered security scanning for GitHub Actions
# 4. Public repository
# 5. Add README
# 6. Choose MIT license
# 7. Create repository
```

### Step 2: Initialize Repository

```bash
# Clone the new repository
cd ~/projects
git clone https://github.com/cloudshipai/station-action.git
cd station-action

# Copy files from template
cp ~/projects/hack/station/dev-workspace/github-action-template/action.yml .
cp ~/projects/hack/station/dev-workspace/github-action-template/README.md .
mkdir examples
cp ~/projects/hack/station/dev-workspace/github-action-template/examples/* examples/

# Create additional required files
```

### Step 3: Create LICENSE File

```bash
cat > LICENSE << 'EOF'
MIT License

Copyright (c) 2025 CloudShip AI

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
EOF
```

### Step 4: Commit and Push

```bash
git add .
git commit -m "Initial release: CloudShip AI Security Scanner action

Features:
- 6 specialized AI security agents
- 97+ integrated security tools
- Automatic PR comments
- Customizable tasks and agents
- Support for infrastructure, code, and supply chain security
"

git push origin main
```

### Step 5: Create Release Tag

```bash
# Create v1.0.0 release
git tag -a v1.0.0 -m "Release v1.0.0: Initial public release"
git push origin v1.0.0

# Create v1 tag (for users who want latest v1.x)
git tag -a v1 -m "v1 latest"
git push origin v1
```

### Step 6: Publish to GitHub Marketplace

#### Via GitHub Web UI:

1. Go to https://github.com/cloudshipai/station-action
2. Click "Releases" → "Create a new release"
3. Tag: `v1.0.0`
4. Release title: "CloudShip AI Security Scanner v1.0.0"
5. Description:
```markdown
## 🚀 Initial Public Release

Add AI-powered security scanning to your GitHub Actions in 3 lines!

### ✨ Features

- 🤖 **6 Specialized AI Agents** - Infrastructure, PR Review, Supply Chain, and more
- 🛠️ **97+ Security Tools** - Checkov, TFLint, Semgrep, Syft, Trivy, etc.
- 🧠 **AI-Powered Analysis** - Intelligent insights, not just tool output
- 📊 **Automatic PR Comments** - Security findings directly in PRs
- 🔧 **Fully Customizable** - Override tasks and workflows

### 📖 Quick Start

```yaml
- uses: actions/checkout@v4
- uses: cloudshipai/station-action@v1
  with:
    agent: infrastructure-security
  env:
    OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

### 📚 Documentation

- [Full README](https://github.com/cloudshipai/station-action#readme)
- [Example Workflows](https://github.com/cloudshipai/station-action/tree/main/examples)
- [CloudShip AI Docs](https://docs.cloudshipai.com)

### 🆕 What's New

Initial release with support for:
- Infrastructure security scanning (Terraform, K8s, Docker)
- PR security reviews
- Supply chain security (SBOM + vulnerability scanning)
- Deployment security gates
- Security improvement recommendations
- Security metrics reporting
```

6. Check ✅ "Publish this Action to the GitHub Marketplace"
7. Select primary category: **Security**
8. Select secondary category: **Code quality**
9. Add icon: `shield`
10. Add color: `blue`
11. Click "Publish release"

### Step 7: Verify Marketplace Listing

1. Go to https://github.com/marketplace/actions
2. Search for "CloudShip AI Security Scanner"
3. Verify:
   - ✅ Action appears in search results
   - ✅ README renders correctly
   - ✅ Examples are visible
   - ✅ Shield icon and blue color are correct
   - ✅ "Use latest version" button works

### Step 8: Test the Action

Create a test repository:

```bash
# Create test repo
gh repo create cloudshipai/station-action-test --public

cd ~/projects
git clone https://github.com/cloudshipai/station-action-test.git
cd station-action-test

# Add some test files
mkdir -p terraform
cat > terraform/main.tf << 'EOF'
resource "aws_s3_bucket" "test" {
  bucket = "test-bucket"
  acl    = "public-read"  # Security issue - public bucket
}

resource "aws_security_group" "test" {
  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]  # Security issue - SSH open to world
  }
}
EOF

# Add workflow
mkdir -p .github/workflows
cat > .github/workflows/security.yml << 'EOF'
name: Security Scan
on: [push, pull_request]

jobs:
  security:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write
    steps:
      - uses: actions/checkout@v4
      - uses: cloudshipai/station-action@v1
        with:
          agent: infrastructure-security
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
EOF

# Add OpenAI API key secret
gh secret set OPENAI_API_KEY --body "your-openai-api-key-here"

# Commit and push
git add .
git commit -m "Add security scanning workflow"
git push origin main
```

Watch the workflow run:
```bash
gh run watch
```

Verify:
- ✅ Workflow runs successfully
- ✅ Security issues are detected (public S3 bucket, open SSH)
- ✅ Logs show AI analysis
- ✅ If PR, comment is posted

### Step 9: Create Marketing Materials

#### Badge for README

Add to project READMEs:
```markdown
[![CloudShip Security](https://img.shields.io/badge/CloudShip-Security%20Enabled-blue?logo=github&logoColor=white)](https://github.com/cloudshipai/station-action)
```

#### Social Media Announcement

**Twitter/X Post:**
```
🚀 Introducing CloudShip AI Security Scanner for GitHub Actions

Add AI-powered security scanning in 3 lines:
- 97+ security tools
- 6 specialized AI agents
- Automatic PR comments

Try it: https://github.com/marketplace/actions/cloudship-ai-security-scanner

#DevSecOps #GitHubActions #AI #Security
```

**LinkedIn Post:**
```
We just launched CloudShip AI Security Scanner on GitHub Marketplace!

Setting up comprehensive security scanning used to take hours:
- Install Snyk
- Install SonarQube
- Install Trivy
- Install Checkov
- Configure each tool
- Integrate results

With CloudShip, it's 3 lines:

```yaml
- uses: cloudshipai/station-action@v1
  with:
    agent: infrastructure-security
```

What makes it different:
✅ 97+ security tools in one integration
✅ AI-powered analysis and prioritization
✅ Works with Terraform, Kubernetes, Docker, and more
✅ Automatic PR comments with actionable insights

Check it out: https://github.com/marketplace/actions/cloudship-ai-security-scanner
```

#### Blog Post Outline

Title: "Add AI Security Scanning to GitHub Actions in 2 Minutes"

1. **The Problem**: Setting up security scanning is too complex
2. **The Solution**: CloudShip AI Station Action
3. **How It Works**: Show 3-line example
4. **Demo**: Real example with test repo
5. **Available Agents**: Explain the 6 agents
6. **Pricing**: Free action + OpenAI API costs
7. **Get Started**: Link to marketplace

### Step 10: Monitor and Iterate

#### Track Metrics:
- GitHub Stars: https://github.com/cloudshipai/station-action/stargazers
- Marketplace installs: GitHub Marketplace dashboard
- Issues: https://github.com/cloudshipai/station-action/issues
- Discussions: Enable GitHub Discussions

#### Iterate Based on Feedback:
- Add requested features
- Improve documentation
- Add more example workflows
- Fix bugs
- Add support for more CICD platforms

## Post-Launch Checklist

- [ ] Repository created and public
- [ ] Files committed and pushed
- [ ] Release created with v1.0.0 tag
- [ ] Published to GitHub Marketplace
- [ ] Verified marketplace listing
- [ ] Tested in real repository
- [ ] Documentation complete
- [ ] Examples provided
- [ ] Marketing materials created
- [ ] Social media posts published
- [ ] Monitoring set up

## Maintenance

### Updating the Action

When making changes:

```bash
# Make changes to action.yml or other files
git add .
git commit -m "Description of changes"
git push origin main

# Create new version
git tag -a v1.0.1 -m "Bug fix: ..."
git push origin v1.0.1

# Update v1 tag to point to latest v1.x
git tag -f v1
git push -f origin v1

# Create GitHub release with changelog
gh release create v1.0.1 \
  --title "v1.0.1: Bug Fixes" \
  --notes "- Fixed XYZ issue\n- Improved ABC"
```

### Supporting Users

1. **GitHub Issues**: Monitor and respond to issues
2. **GitHub Discussions**: Answer questions
3. **Discord**: Create support channel
4. **Documentation**: Keep README updated

## Success Metrics

Target goals:
- **Week 1**: 100 installations
- **Month 1**: 1,000 installations
- **Month 3**: 5,000 installations
- **GitHub Stars**: 500+ in first month

Track:
- Installation count (Marketplace insights)
- Star growth (GitHub stats)
- Issue/discussion engagement
- Social media mentions
- Blog post views

## Next Steps After Launch

1. **GitHub App**: Build web-based configuration UI
2. **More Bundles**: Add FinOps and DevOps bundles
3. **Custom Bundles**: Let users bring their own
4. **GitLab CI**: Create GitLab CI adapter
5. **CircleCI**: Create CircleCI orb
6. **Dagger Module**: Publish to Daggerverse
7. **Self-Hosted**: Add option to run without API key

---

**Ready to deploy? Let's ship it! 🚀**
