# Supermetrics CLI Examples: Query Marketing Data from 200+ Sources

> Command-line examples for fetching data from Google Ads, Facebook Ads, Google Analytics 4,
> LinkedIn Ads, TikTok Ads, Microsoft Advertising, Shopify, HubSpot, Salesforce, YouTube,
> Instagram, Ahrefs, SEMrush, Stripe, and 200+ other marketing and analytics platforms
> using the `supermetrics` CLI.

This guide provides copy-paste-ready commands for common marketing data tasks. Each example uses
the `supermetrics queries execute` command with real data source IDs from the
[Supermetrics API](https://supermetrics.com).

> **Note:** Available metrics and dimensions vary by data source and account. Use
> `supermetrics datasource get --team-id <ID> --data-source-id <DS_ID>` to inspect what fields
> are available for a given source. The field names in these examples are representative —
> consult your data source's documentation for the exact field names available to your account.

Back to [README](../README.md)

---

## Table of Contents

- [Prerequisites](#prerequisites)
- **Getting Started**
  - [How do I authenticate with the Supermetrics CLI?](#how-do-i-authenticate-with-the-supermetrics-cli)
  - [How do I find available accounts and data sources?](#how-do-i-find-available-accounts-and-data-sources)
- **Web Analytics**
  - [How do I pull Google Analytics 4 session and pageview data?](#how-do-i-pull-google-analytics-4-session-and-pageview-data)
  - [How do I get Google Analytics traffic by source, medium, or campaign?](#how-do-i-get-google-analytics-traffic-by-source-medium-or-campaign)
  - [How do I query Adobe Analytics data from the command line?](#how-do-i-query-adobe-analytics-data-from-the-command-line)
  - [How do I get Google Search Console keyword and page performance data?](#how-do-i-get-google-search-console-keyword-and-page-performance-data)
  - [How do I fetch Matomo (Piwik) analytics data?](#how-do-i-fetch-matomo-piwik-analytics-data)
- **Paid Advertising**
  - [How do I pull Google Ads campaign performance data?](#how-do-i-pull-google-ads-campaign-performance-data)
  - [How do I get Facebook Ads (Meta Ads) spend and ROAS data?](#how-do-i-get-facebook-ads-meta-ads-spend-and-roas-data)
  - [How do I fetch LinkedIn Ads campaign metrics?](#how-do-i-fetch-linkedin-ads-campaign-metrics)
  - [How do I pull Microsoft Advertising (Bing Ads) performance data?](#how-do-i-pull-microsoft-advertising-bing-ads-performance-data)
  - [How do I get TikTok Ads campaign performance data?](#how-do-i-get-tiktok-ads-campaign-performance-data)
  - [How do I query X (Twitter) Ads performance?](#how-do-i-query-x-twitter-ads-performance)
  - [How do I pull Amazon Ads campaign data?](#how-do-i-pull-amazon-ads-campaign-data)
  - [How do I get Snapchat Ads performance metrics?](#how-do-i-get-snapchat-ads-performance-metrics)
  - [How do I query Reddit Ads campaign data?](#how-do-i-query-reddit-ads-campaign-data)
  - [How do I pull Pinterest Ads performance data?](#how-do-i-pull-pinterest-ads-performance-data)
  - [How do I get Search Ads 360 campaign performance?](#how-do-i-get-search-ads-360-campaign-performance)
  - [How do I pull Google Display & Video 360 (DV360) data?](#how-do-i-pull-google-display--video-360-dv360-data)
  - [How do I get Criteo advertising performance data?](#how-do-i-get-criteo-advertising-performance-data)
- **Social Media (Organic)**
  - [How do I get Facebook Page organic reach and engagement?](#how-do-i-get-facebook-page-organic-reach-and-engagement)
  - [How do I pull Instagram Insights organic performance data?](#how-do-i-pull-instagram-insights-organic-performance-data)
  - [How do I query YouTube channel analytics?](#how-do-i-query-youtube-channel-analytics)
  - [How do I get LinkedIn Page organic analytics?](#how-do-i-get-linkedin-page-organic-analytics)
  - [How do I pull TikTok organic account analytics?](#how-do-i-pull-tiktok-organic-account-analytics)
  - [How do I get X (Twitter) organic tweet analytics?](#how-do-i-get-x-twitter-organic-tweet-analytics)
  - [How do I get Threads Insights data?](#how-do-i-get-threads-insights-data)
- **SEO Tools**
  - [How do I pull Ahrefs backlink and keyword data?](#how-do-i-pull-ahrefs-backlink-and-keyword-data)
  - [How do I query SEMrush keyword rankings?](#how-do-i-query-semrush-keyword-rankings)
  - [How do I query Google Trends search interest data?](#how-do-i-query-google-trends-search-interest-data)
  - [How do I get Similarweb traffic analytics?](#how-do-i-get-similarweb-traffic-analytics)
- **E-commerce**
  - [How do I pull Shopify sales and order data?](#how-do-i-pull-shopify-sales-and-order-data)
  - [How do I get WooCommerce order analytics?](#how-do-i-get-woocommerce-order-analytics)
  - [How do I query Stripe payment data?](#how-do-i-query-stripe-payment-data)
- **Email Marketing**
  - [How do I query Mailchimp email campaign performance?](#how-do-i-query-mailchimp-email-campaign-performance)
  - [How do I pull Klaviyo email and SMS campaign metrics?](#how-do-i-pull-klaviyo-email-and-sms-campaign-metrics)
  - [How do I get ActiveCampaign email performance data?](#how-do-i-get-activecampaign-email-performance-data)
  - [How do I query Brevo (Sendinblue) campaign data?](#how-do-i-query-brevo-sendinblue-campaign-data)
- **CRM & Marketing Automation**
  - [How do I query Salesforce CRM data from the command line?](#how-do-i-query-salesforce-crm-data-from-the-command-line)
  - [How do I pull HubSpot CRM and marketing data?](#how-do-i-pull-hubspot-crm-and-marketing-data)
  - [How do I get Marketo marketing automation data?](#how-do-i-get-marketo-marketing-automation-data)
  - [How do I query Pipedrive CRM data?](#how-do-i-query-pipedrive-crm-data)
- **Cross-Platform Comparisons**
  - [How do I compare ad spend across Google, Facebook, and LinkedIn?](#how-do-i-compare-ad-spend-across-google-facebook-and-linkedin)
  - [How do I compare organic social performance across platforms?](#how-do-i-compare-organic-social-performance-across-platforms)
- **Output Formatting & Data Processing**
  - [How do I export Supermetrics data to CSV?](#how-do-i-export-supermetrics-data-to-csv)
  - [How do I format query results as a readable table?](#how-do-i-format-query-results-as-a-readable-table)
  - [How do I pipe Supermetrics CLI output to jq?](#how-do-i-pipe-supermetrics-cli-output-to-jq)
- **Filtering, Sorting & Pagination**
  - [How do I filter query results in the Supermetrics CLI?](#how-do-i-filter-query-results-in-the-supermetrics-cli)
  - [How do I sort results by a metric?](#how-do-i-sort-results-by-a-metric)
  - [How do I paginate through large result sets?](#how-do-i-paginate-through-large-result-sets)
- **Data Warehouse Backfills**
  - [How do I create a Data Warehouse backfill?](#how-do-i-create-a-data-warehouse-backfill)
  - [How do I check backfill status or cancel a backfill?](#how-do-i-check-backfill-status-or-cancel-a-backfill)
- **Login Links & Auth Management**
  - [How do I create and manage OAuth login links?](#how-do-i-create-and-manage-oauth-login-links)
- **Scripting & Automation**
  - [How do I use the Supermetrics CLI in a bash script or cron job?](#how-do-i-use-the-supermetrics-cli-in-a-bash-script-or-cron-job)
  - [How do I use environment variables and profiles in CI/CD?](#how-do-i-use-environment-variables-and-profiles-in-cicd)
- **Troubleshooting**
  - [How do I debug queries that return errors?](#how-do-i-debug-queries-that-return-errors)
  - [How do I set custom timeouts for slow queries?](#how-do-i-set-custom-timeouts-for-slow-queries)
- [Data Source ID Quick Reference](#data-source-id-quick-reference)

---

## Prerequisites

Before running any examples, authenticate with your Supermetrics account:

```bash
supermetrics login          # OAuth login (recommended)
# or
supermetrics configure      # Set up an API key
```

---

## Getting Started

### How do I authenticate with the Supermetrics CLI?

The Supermetrics CLI supports OAuth login and API key authentication. OAuth is
recommended for interactive use; API keys are better for scripts and automation.

**OAuth login (recommended):**

```bash
supermetrics login
```

This opens your browser for Google or Microsoft sign-in. Tokens are stored locally
and refreshed automatically.

**API key setup:**

```bash
supermetrics configure
```

Or pass the key directly:

```bash
export SUPERMETRICS_API_KEY=your-api-key-here
```

**Multiple accounts with named profiles:**

```bash
supermetrics configure --profile client-a
supermetrics login --profile client-b

supermetrics profile list          # Show all profiles (* = active)
supermetrics profile use client-a  # Switch active profile

# Run a one-off command with a specific profile
supermetrics queries execute --profile client-b --ds-id GAWA --fields sessions \
  --start-date 30-days-ago --end-date yesterday
```

---

### How do I find available accounts and data sources?

Retrieve the list of accounts connected to a data source, or inspect a data source's
available fields and configuration.

**List accounts for Google Analytics 4:**

```bash
supermetrics accounts list --ds-id GAWA -o table
```

**List accounts for Facebook Ads (Meta Ads):**

```bash
supermetrics accounts list --ds-id FA -o table
```

**Export all Google Ads accounts to CSV:**

```bash
supermetrics accounts list --ds-id AW -o csv > google-ads-accounts.csv
```

**View all authenticated logins:**

```bash
supermetrics logins list -o table --flatten
```

**Inspect data source configuration (available fields, report types):**

```bash
supermetrics datasource get --team-id 123 --data-source-id GAWA
supermetrics datasource get --team-id 123 --data-source-id FA \
  --fields name,status,categories -o table
```

---

## Web Analytics

### How do I pull Google Analytics 4 session and pageview data?

Fetch GA4 session metrics for a specific date range using data source `GAWA`
(Google Analytics 4).

```bash
supermetrics queries execute \
  --ds-id GAWA \
  --fields sessions \
  --fields users \
  --fields screen_page_views \
  --start-date 2025-01-01 \
  --end-date 2025-01-31
```

**With a specific GA4 property and CSV output:**

```bash
supermetrics queries execute \
  --ds-id GAWA \
  --fields date \
  --fields sessions \
  --fields users \
  --fields screen_page_views \
  --fields bounce_rate \
  --ds-accounts "properties/123456789" \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o csv > ga4-sessions.csv
```

- `--ds-id GAWA` targets Google Analytics 4
- `--ds-accounts` narrows to a specific GA4 property
- Relative dates like `30-days-ago` and `yesterday` avoid hardcoding

---

### How do I get Google Analytics traffic by source, medium, or campaign?

Break down GA4 traffic by acquisition source and medium to see where visitors
come from.

```bash
supermetrics queries execute \
  --ds-id GAWA \
  --fields session_source \
  --fields session_medium \
  --fields sessions \
  --fields users \
  --fields conversions \
  --start-date 30-days-ago \
  --end-date yesterday \
  --order-rows "sessions desc" \
  -o table
```

**Traffic by UTM campaign name:**

```bash
supermetrics queries execute \
  --ds-id GAWA \
  --fields session_campaign_name \
  --fields sessions \
  --fields users \
  --fields engagement_rate \
  --start-date 30-days-ago \
  --end-date yesterday \
  --max-rows 50 \
  -o table
```

---

### How do I query Adobe Analytics data from the command line?

Pull visitor and pageview data from Adobe Analytics 2.0 (`ADA`) or Adobe Analytics
legacy (`ASC`).

```bash
supermetrics queries execute \
  --ds-id ADA \
  --fields visits \
  --fields visitors \
  --fields page_views \
  --fields date \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

**With a specific report suite:**

```bash
supermetrics queries execute \
  --ds-id ADA \
  --fields visits \
  --fields page_views \
  --fields bounce_rate \
  --ds-accounts "your-report-suite-id" \
  --start-date 2025-01-01 \
  --end-date 2025-03-31 \
  -o csv > adobe-analytics.csv
```

---

### How do I get Google Search Console keyword and page performance data?

Retrieve search performance data from Google Search Console (`GW`) — clicks,
impressions, CTR, and average position by query or page.

**Top search queries by clicks:**

```bash
supermetrics queries execute \
  --ds-id GW \
  --fields query \
  --fields clicks \
  --fields impressions \
  --fields ctr \
  --fields position \
  --start-date 30-days-ago \
  --end-date yesterday \
  --order-rows "clicks desc" \
  --max-rows 100 \
  -o table
```

**Page-level performance:**

```bash
supermetrics queries execute \
  --ds-id GW \
  --fields page \
  --fields clicks \
  --fields impressions \
  --fields ctr \
  --start-date 30-days-ago \
  --end-date yesterday \
  --order-rows "impressions desc" \
  -o csv > gsc-pages.csv
```

---

### How do I fetch Matomo (Piwik) analytics data?

Query self-hosted Matomo analytics (`MATO`) for visit and pageview metrics.

```bash
supermetrics queries execute \
  --ds-id MATO \
  --fields date \
  --fields visits \
  --fields unique_visitors \
  --fields pageviews \
  --fields bounce_rate \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

## Paid Advertising

### How do I pull Google Ads campaign performance data?

Retrieve Google Ads (`AW`) campaign metrics — clicks, impressions, cost,
and conversions.

```bash
supermetrics queries execute \
  --ds-id AW \
  --fields campaign_name \
  --fields clicks \
  --fields impressions \
  --fields cost \
  --fields conversions \
  --start-date 30-days-ago \
  --end-date yesterday \
  --order-rows "cost desc" \
  -o table
```

**Export Google Ads data to CSV for spreadsheet analysis:**

```bash
supermetrics queries execute \
  --ds-id AW \
  --fields date \
  --fields campaign_name \
  --fields clicks \
  --fields impressions \
  --fields cost \
  --fields conversions \
  --fields ctr \
  --fields average_cpc \
  --start-date 2025-01-01 \
  --end-date 2025-03-31 \
  --max-rows 10000 \
  -o csv > google-ads-campaigns.csv
```

**Google Ads with specific account targeting:**

```bash
supermetrics queries execute \
  --ds-id AW \
  --fields campaign_name \
  --fields clicks \
  --fields cost \
  --ds-accounts "123-456-7890" \
  --start-date 30-days-ago \
  --end-date yesterday
```

---

### How do I get Facebook Ads (Meta Ads) spend and ROAS data?

Pull Facebook Ads (`FA`) — also known as Meta Ads — campaign spend, impressions,
clicks, and return on ad spend.

```bash
supermetrics queries execute \
  --ds-id FA \
  --fields campaign_name \
  --fields spend \
  --fields impressions \
  --fields clicks \
  --fields cpc \
  --fields cpm \
  --fields purchase_roas \
  --start-date 30-days-ago \
  --end-date yesterday \
  --order-rows "spend desc" \
  -o table
```

**Facebook Ads daily breakdown for a specific ad account:**

```bash
supermetrics queries execute \
  --ds-id FA \
  --fields date \
  --fields spend \
  --fields impressions \
  --fields clicks \
  --fields conversions \
  --ds-accounts "act_123456789" \
  --start-date 2025-01-01 \
  --end-date 2025-01-31 \
  -o csv > facebook-ads-daily.csv
```

---

### How do I fetch LinkedIn Ads campaign metrics?

Retrieve LinkedIn Ads (`LIA`) campaign performance — impressions, clicks, spend,
and conversions for B2B advertising.

```bash
supermetrics queries execute \
  --ds-id LIA \
  --fields campaign_name \
  --fields impressions \
  --fields clicks \
  --fields cost \
  --fields conversions \
  --fields ctr \
  --start-date 30-days-ago \
  --end-date yesterday \
  --order-rows "cost desc" \
  -o table
```

---

### How do I pull Microsoft Advertising (Bing Ads) performance data?

Fetch campaign metrics from Microsoft Advertising (`AC`), formerly known as Bing Ads.

```bash
supermetrics queries execute \
  --ds-id AC \
  --fields campaign_name \
  --fields clicks \
  --fields impressions \
  --fields spend \
  --fields conversions \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

**Export to CSV:**

```bash
supermetrics queries execute \
  --ds-id AC \
  --fields date \
  --fields campaign_name \
  --fields clicks \
  --fields impressions \
  --fields spend \
  --start-date 2025-01-01 \
  --end-date 2025-03-31 \
  -o csv > bing-ads.csv
```

---

### How do I get TikTok Ads campaign performance data?

Pull TikTok Ads (`TIK`) campaign metrics — impressions, clicks, spend,
and video views.

```bash
supermetrics queries execute \
  --ds-id TIK \
  --fields campaign_name \
  --fields impressions \
  --fields clicks \
  --fields spend \
  --fields video_views \
  --fields conversions \
  --start-date 30-days-ago \
  --end-date yesterday \
  --order-rows "spend desc" \
  -o table
```

---

### How do I query X (Twitter) Ads performance?

Fetch X Ads (`TA`) — formerly Twitter Ads — campaign engagement and spend data.

```bash
supermetrics queries execute \
  --ds-id TA \
  --fields campaign_name \
  --fields impressions \
  --fields engagements \
  --fields spend \
  --fields clicks \
  --fields engagement_rate \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

### How do I pull Amazon Ads campaign data?

Retrieve Amazon Ads (`AA`) Sponsored Products, Brands, and Display campaign
performance.

```bash
supermetrics queries execute \
  --ds-id AA \
  --fields campaign_name \
  --fields impressions \
  --fields clicks \
  --fields spend \
  --fields acos \
  --fields roas \
  --start-date 30-days-ago \
  --end-date yesterday \
  --order-rows "spend desc" \
  -o table
```

---

### How do I get Snapchat Ads performance metrics?

Pull Snapchat Marketing (`SCM`) campaign metrics including swipe-ups and spend.

```bash
supermetrics queries execute \
  --ds-id SCM \
  --fields campaign_name \
  --fields impressions \
  --fields swipes \
  --fields spend \
  --fields video_views \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

### How do I query Reddit Ads campaign data?

Fetch Reddit Ads (`RDA`) campaign performance — impressions, clicks, and spend.

```bash
supermetrics queries execute \
  --ds-id RDA \
  --fields campaign_name \
  --fields impressions \
  --fields clicks \
  --fields spend \
  --fields cpc \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

### How do I pull Pinterest Ads performance data?

Retrieve Pinterest Ads (`PIA`) campaign metrics for promoted pins.

```bash
supermetrics queries execute \
  --ds-id PIA \
  --fields campaign_name \
  --fields impressions \
  --fields clicks \
  --fields spend \
  --fields conversions \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

### How do I get Search Ads 360 campaign performance?

Pull Google Search Ads 360 (`SA360`) enterprise search campaign data.

```bash
supermetrics queries execute \
  --ds-id SA360 \
  --fields campaign_name \
  --fields clicks \
  --fields impressions \
  --fields cost \
  --fields conversions \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

### How do I pull Google Display & Video 360 (DV360) data?

Fetch Google Display & Video 360 (`DBM`) programmatic campaign performance.

```bash
supermetrics queries execute \
  --ds-id DBM \
  --fields insertion_order_name \
  --fields impressions \
  --fields clicks \
  --fields total_media_cost \
  --fields total_conversions \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

### How do I get Criteo advertising performance data?

Retrieve Criteo (`CRI`) retargeting and display campaign metrics.

```bash
supermetrics queries execute \
  --ds-id CRI \
  --fields campaign_name \
  --fields impressions \
  --fields clicks \
  --fields cost \
  --fields sales \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

## Social Media (Organic)

### How do I get Facebook Page organic reach and engagement?

Pull organic Facebook Page metrics from Facebook Insights (`FB`) —
reach, impressions, and engagement.

```bash
supermetrics queries execute \
  --ds-id FB \
  --fields date \
  --fields page_impressions \
  --fields page_engaged_users \
  --fields page_fan_adds \
  --fields page_views_total \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

**Post-level metrics with auto-pagination:**

```bash
supermetrics queries execute \
  --ds-id FB \
  --fields post_message \
  --fields post_impressions \
  --fields post_engaged_users \
  --fields post_clicks \
  --start-date 30-days-ago \
  --end-date yesterday \
  --all \
  -o csv > facebook-posts.csv
```

---

### How do I pull Instagram Insights organic performance data?

Fetch organic Instagram Insights (`IGI`) — reach, impressions, and
engagement metrics for your Instagram business account.

```bash
supermetrics queries execute \
  --ds-id IGI \
  --fields date \
  --fields impressions \
  --fields reach \
  --fields profile_views \
  --fields follower_count \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

### How do I query YouTube channel analytics?

Retrieve YouTube (`YT2`) channel performance — views, watch time,
subscribers, and engagement.

```bash
supermetrics queries execute \
  --ds-id YT2 \
  --fields date \
  --fields views \
  --fields estimated_minutes_watched \
  --fields subscribers_gained \
  --fields likes \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

**Video-level breakdown:**

```bash
supermetrics queries execute \
  --ds-id YT2 \
  --fields video_title \
  --fields views \
  --fields estimated_minutes_watched \
  --fields average_view_duration \
  --fields likes \
  --start-date 30-days-ago \
  --end-date yesterday \
  --order-rows "views desc" \
  --max-rows 50 \
  -o table
```

---

### How do I get LinkedIn Page organic analytics?

Pull LinkedIn Company Page (`LIP`) organic metrics — impressions,
clicks, followers, and engagement.

```bash
supermetrics queries execute \
  --ds-id LIP \
  --fields date \
  --fields impressions \
  --fields clicks \
  --fields engagement_rate \
  --fields follower_count \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

### How do I pull TikTok organic account analytics?

Fetch TikTok Organic (`TIKBA`) account-level and video-level performance data.

```bash
supermetrics queries execute \
  --ds-id TIKBA \
  --fields date \
  --fields video_views \
  --fields likes \
  --fields shares \
  --fields comments \
  --fields followers_count \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

### How do I get X (Twitter) organic tweet analytics?

Retrieve X Organic (`TWO`) — formerly Twitter — tweet performance,
impressions, and engagement metrics.

```bash
supermetrics queries execute \
  --ds-id TWO \
  --fields date \
  --fields impressions \
  --fields engagements \
  --fields retweets \
  --fields likes \
  --fields replies \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

### How do I get Threads Insights data?

Pull Meta Threads (`THRDS`) post performance and engagement metrics.

```bash
supermetrics queries execute \
  --ds-id THRDS \
  --fields date \
  --fields views \
  --fields likes \
  --fields replies \
  --fields reposts \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

## SEO Tools

### How do I pull Ahrefs backlink and keyword data?

Retrieve Ahrefs (`AHRF2`) SEO metrics — backlink data, referring domains,
and keyword rankings.

```bash
supermetrics queries execute \
  --ds-id AHRF2 \
  --fields date \
  --fields referring_domains \
  --fields backlinks \
  --fields domain_rating \
  --fields organic_traffic \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

### How do I query SEMrush keyword rankings?

Fetch SEMrush Analytics (`SR`) keyword position and organic traffic data
for competitive analysis and SEO tracking.

```bash
supermetrics queries execute \
  --ds-id SR \
  --fields keyword \
  --fields position \
  --fields search_volume \
  --fields traffic \
  --fields url \
  --start-date 30-days-ago \
  --end-date yesterday \
  --max-rows 200 \
  -o csv > semrush-keywords.csv
```

**SEMrush Projects (`SRP`) for tracked keyword positions:**

```bash
supermetrics queries execute \
  --ds-id SRP \
  --fields keyword \
  --fields position \
  --fields previous_position \
  --fields search_volume \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

### How do I query Google Trends search interest data?

Pull Google Trends (`GT`) search interest over time for keyword research and trend
analysis.

```bash
supermetrics queries execute \
  --ds-id GT \
  --fields keyword \
  --fields interest_over_time \
  --start-date 2024-01-01 \
  --end-date 2025-01-01 \
  -o table
```

---

### How do I get Similarweb traffic analytics?

Fetch Similarweb (`SW`) website traffic estimates and competitive intelligence.

```bash
supermetrics queries execute \
  --ds-id SW \
  --fields date \
  --fields total_visits \
  --fields desktop_visits \
  --fields mobile_visits \
  --fields bounce_rate \
  --fields pages_per_visit \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

## E-commerce

### How do I pull Shopify sales and order data?

Retrieve Shopify (`SHP`) e-commerce data — orders, revenue, and
product performance from the command line.

```bash
supermetrics queries execute \
  --ds-id SHP \
  --fields date \
  --fields orders \
  --fields gross_sales \
  --fields net_sales \
  --fields average_order_value \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

**Product-level breakdown with flattened table:**

```bash
supermetrics queries execute \
  --ds-id SHP \
  --fields product_title \
  --fields quantity \
  --fields gross_sales \
  --fields net_sales \
  --start-date 30-days-ago \
  --end-date yesterday \
  --order-rows "gross_sales desc" \
  --max-rows 50 \
  -o table --flatten
```

---

### How do I get WooCommerce order analytics?

Fetch WooCommerce (`WOOC`) order and revenue metrics from your WordPress store.

```bash
supermetrics queries execute \
  --ds-id WOOC \
  --fields date \
  --fields orders \
  --fields total_sales \
  --fields average_order_value \
  --fields items_sold \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

### How do I query Stripe payment data?

Pull Stripe (`ST`) payment and revenue data from the command line.

```bash
supermetrics queries execute \
  --ds-id ST \
  --fields date \
  --fields gross_volume \
  --fields net_volume \
  --fields successful_payments \
  --fields refunds \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

## Email Marketing

### How do I query Mailchimp email campaign performance?

Retrieve Mailchimp (`MC`) email campaign metrics — opens, clicks,
unsubscribes, and delivery rates.

```bash
supermetrics queries execute \
  --ds-id MC \
  --fields campaign_name \
  --fields emails_sent \
  --fields opens \
  --fields clicks \
  --fields unsubscribes \
  --fields open_rate \
  --fields click_rate \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

### How do I pull Klaviyo email and SMS campaign metrics?

Fetch Klaviyo (`KLAV`) email and SMS campaign data — deliveries, opens, clicks,
and attributed revenue for e-commerce email marketing.

```bash
supermetrics queries execute \
  --ds-id KLAV \
  --fields campaign_name \
  --fields emails_delivered \
  --fields opens \
  --fields clicks \
  --fields revenue \
  --fields unsubscribes \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

### How do I get ActiveCampaign email performance data?

Retrieve ActiveCampaign (`ACT`) email automation and campaign metrics.

```bash
supermetrics queries execute \
  --ds-id ACT \
  --fields campaign_name \
  --fields sends \
  --fields opens \
  --fields clicks \
  --fields bounces \
  --fields unsubscribes \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

### How do I query Brevo (Sendinblue) campaign data?

Pull Brevo (`SIB`) — formerly Sendinblue — email campaign performance data.

```bash
supermetrics queries execute \
  --ds-id SIB \
  --fields campaign_name \
  --fields delivered \
  --fields opens \
  --fields clicks \
  --fields unsubscribes \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

## CRM & Marketing Automation

### How do I query Salesforce CRM data from the command line?

Retrieve Salesforce (`SF`) CRM data — leads, opportunities, pipeline value,
and deal stages.

```bash
supermetrics queries execute \
  --ds-id SF \
  --fields stage_name \
  --fields opportunity_count \
  --fields amount \
  --fields close_date \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

**Salesforce leads by source:**

```bash
supermetrics queries execute \
  --ds-id SF \
  --fields lead_source \
  --fields lead_count \
  --fields converted_count \
  --fields conversion_rate \
  --start-date 2025-01-01 \
  --end-date 2025-03-31 \
  -o csv > salesforce-leads.csv
```

---

### How do I pull HubSpot CRM and marketing data?

Fetch HubSpot (`HS`) CRM contacts, deals, and marketing analytics from
the command line.

```bash
supermetrics queries execute \
  --ds-id HS \
  --fields date \
  --fields contacts_created \
  --fields deals_created \
  --fields deals_won \
  --fields revenue \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

**HubSpot marketing email performance (`HSME`):**

```bash
supermetrics queries execute \
  --ds-id HSME \
  --fields email_name \
  --fields sent \
  --fields delivered \
  --fields opens \
  --fields clicks \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

### How do I get Marketo marketing automation data?

Retrieve Marketo (`MARK`) lead and campaign performance data for B2B
marketing automation.

```bash
supermetrics queries execute \
  --ds-id MARK \
  --fields date \
  --fields leads_created \
  --fields email_opens \
  --fields email_clicks \
  --fields form_submissions \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

### How do I query Pipedrive CRM data?

Pull Pipedrive (`PIPE`) sales pipeline and deal data from the command line.

```bash
supermetrics queries execute \
  --ds-id PIPE \
  --fields stage_name \
  --fields deals_count \
  --fields deals_value \
  --fields deals_won \
  --start-date 30-days-ago \
  --end-date yesterday \
  -o table
```

---

## Cross-Platform Comparisons

### How do I compare ad spend across Google, Facebook, and LinkedIn?

Run separate queries for each platform and export to CSV files for side-by-side
comparison in a spreadsheet.

```bash
# Google Ads spend
supermetrics queries execute \
  --ds-id AW \
  --fields date --fields campaign_name --fields cost --fields clicks --fields impressions \
  --start-date 30-days-ago --end-date yesterday \
  -o csv > google-ads-spend.csv

# Facebook Ads spend
supermetrics queries execute \
  --ds-id FA \
  --fields date --fields campaign_name --fields spend --fields clicks --fields impressions \
  --start-date 30-days-ago --end-date yesterday \
  -o csv > facebook-ads-spend.csv

# LinkedIn Ads spend
supermetrics queries execute \
  --ds-id LIA \
  --fields date --fields campaign_name --fields cost --fields clicks --fields impressions \
  --start-date 30-days-ago --end-date yesterday \
  -o csv > linkedin-ads-spend.csv
```

**Run all three in parallel using shell background jobs:**

```bash
supermetrics queries execute --ds-id AW --fields date,campaign_name,cost,clicks \
  --start-date 30-days-ago --end-date yesterday -o csv > google.csv &
supermetrics queries execute --ds-id FA --fields date,campaign_name,spend,clicks \
  --start-date 30-days-ago --end-date yesterday -o csv > facebook.csv &
supermetrics queries execute --ds-id LIA --fields date,campaign_name,cost,clicks \
  --start-date 30-days-ago --end-date yesterday -o csv > linkedin.csv &
wait
echo "All exports complete"
```

---

### How do I compare organic social performance across platforms?

Export organic metrics from Facebook, Instagram, and LinkedIn to compare
engagement across channels.

```bash
# Facebook Page organic
supermetrics queries execute --ds-id FB \
  --fields date --fields page_impressions --fields page_engaged_users \
  --start-date 30-days-ago --end-date yesterday -o csv > fb-organic.csv

# Instagram organic
supermetrics queries execute --ds-id IGI \
  --fields date --fields impressions --fields reach --fields follower_count \
  --start-date 30-days-ago --end-date yesterday -o csv > ig-organic.csv

# LinkedIn Page organic
supermetrics queries execute --ds-id LIP \
  --fields date --fields impressions --fields clicks --fields follower_count \
  --start-date 30-days-ago --end-date yesterday -o csv > li-organic.csv
```

---

## Output Formatting & Data Processing

### How do I export Supermetrics data to CSV?

Add `-o csv` to any query and redirect to a file. CSV output automatically
flattens nested data — no extra flags needed.

```bash
supermetrics queries execute \
  --ds-id GAWA \
  --fields date --fields sessions --fields users --fields conversions \
  --start-date 2025-01-01 --end-date 2025-03-31 \
  -o csv > ga4-data.csv
```

**Large dataset with auto-pagination:**

```bash
supermetrics queries execute \
  --ds-id AW \
  --fields date --fields campaign_name --fields clicks --fields cost \
  --start-date 2025-01-01 --end-date 2025-03-31 \
  --all \
  -o csv > google-ads-all-data.csv
```

---

### How do I format query results as a readable table?

Use `-o table` for human-readable output with box-drawing borders and colored
headers.

```bash
supermetrics queries execute \
  --ds-id FA \
  --fields campaign_name --fields spend --fields clicks --fields cpc \
  --start-date 30-days-ago --end-date yesterday \
  --order-rows "spend desc" \
  --max-rows 20 \
  -o table
```

**Flatten nested data in table output:**

```bash
supermetrics accounts list --ds-id GAWA -o table --flatten
```

---

### How do I pipe Supermetrics CLI output to jq?

The default JSON output pipes directly to `jq` for filtering and transformation.

**Extract specific fields with jq:**

```bash
supermetrics queries execute \
  --ds-id AW \
  --fields campaign_name --fields cost --fields conversions \
  --start-date 30-days-ago --end-date yesterday \
  | jq '[.[] | {campaign: .campaign_name, cost: .cost, conversions: .conversions}]'
```

**Filter to high-spend campaigns:**

```bash
supermetrics queries execute \
  --ds-id FA \
  --fields campaign_name --fields spend --fields roas \
  --start-date 30-days-ago --end-date yesterday \
  | jq '[.[] | select(.spend > 1000)]'
```

**Count results:**

```bash
supermetrics queries execute \
  --ds-id AW \
  --fields campaign_name --fields clicks \
  --start-date 30-days-ago --end-date yesterday \
  | jq length
```

---

## Filtering, Sorting & Pagination

### How do I filter query results in the Supermetrics CLI?

Use `--filter` to apply server-side filters to your query results.

```bash
supermetrics queries execute \
  --ds-id AW \
  --fields campaign_name --fields clicks --fields cost \
  --start-date 30-days-ago --end-date yesterday \
  --filter "cost > 100"
```

**Use `--fields` (global flag) for client-side field selection:**

```bash
supermetrics logins list --fields login_id,ds_info.name,display_name -o table
```

---

### How do I sort results by a metric?

Use `--order-rows` to sort query results by one or more fields.

```bash
# Sort by cost descending (highest spend first)
supermetrics queries execute \
  --ds-id AW \
  --fields campaign_name --fields cost --fields clicks \
  --start-date 30-days-ago --end-date yesterday \
  --order-rows "cost desc"

# Sort by date ascending
supermetrics queries execute \
  --ds-id GAWA \
  --fields date --fields sessions --fields users \
  --start-date 30-days-ago --end-date yesterday \
  --order-rows "date asc"
```

---

### How do I paginate through large result sets?

Use `--all` to automatically fetch every page, or `--limit` to cap the total
number of rows returned.

```bash
# Fetch all pages automatically
supermetrics queries execute \
  --ds-id GAWA \
  --fields date --fields sessions \
  --start-date 2024-01-01 --end-date 2024-12-31 \
  --all

# Fetch up to 500 rows
supermetrics queries execute \
  --ds-id AW \
  --fields campaign_name --fields clicks --fields cost \
  --start-date 30-days-ago --end-date yesterday \
  --limit 500

# Limit both server-side and client-side for faster queries
supermetrics queries execute \
  --ds-id FA \
  --fields campaign_name --fields spend \
  --start-date 30-days-ago --end-date yesterday \
  --limit 500 --max-rows 500
```

---

## Data Warehouse Backfills

### How do I create a Data Warehouse backfill?

Re-process historical data for a Supermetrics Data Warehouse transfer using
`backfills create`. Specify the team, transfer, and date range.

```bash
supermetrics backfills create \
  --team-id 1 \
  --transfer-id 2 \
  --range-start 2025-01-01 \
  --range-end 2025-01-31
```

**Wait for completion with a progress bar:**

```bash
supermetrics backfills create \
  --team-id 1 \
  --transfer-id 2 \
  --range-start 2025-01-01 \
  --range-end 2025-03-31 \
  --wait
```

**Preview the request without executing (dry-run):**

```bash
supermetrics backfills create \
  --team-id 1 \
  --transfer-id 2 \
  --range-start 2025-01-01 \
  --range-end 2025-01-31 \
  --dry-run
```

---

### How do I check backfill status or cancel a backfill?

Monitor, inspect, or cancel Data Warehouse backfills.

**Get a specific backfill's status:**

```bash
supermetrics backfills get --team-id 1 --backfill-id 12345
```

**Get the latest backfill for a transfer:**

```bash
supermetrics backfills get-latest --team-id 1 --transfer-id 2
```

**List all incomplete backfills for a team:**

```bash
supermetrics backfills list-incomplete --team-id 1 -o table
```

**Cancel a running backfill:**

```bash
supermetrics backfills cancel --team-id 1 --backfill-id 12345
supermetrics backfills cancel --team-id 1 --backfill-id 12345 --yes  # skip confirmation
```

---

## Login Links & Auth Management

### How do I create and manage OAuth login links?

Create shareable login links for data source authentication — useful for
onboarding team members or clients who need to connect their accounts.

**Create a login link for Google Analytics 4:**

```bash
supermetrics login-links create \
  --ds-id GAWA \
  --expiry-time 2025-12-31T23:59:59Z \
  --description "GA4 login link for marketing team"
```

**Preview the request without creating the link:**

```bash
supermetrics login-links create \
  --ds-id FA \
  --expiry-time 2025-12-31T23:59:59Z \
  --dry-run
```

**List all login links:**

```bash
supermetrics login-links list -o table
```

**Close (deactivate) a login link:**

```bash
supermetrics login-links close --yes
```

---

## Scripting & Automation

### How do I use the Supermetrics CLI in a bash script or cron job?

The CLI is designed for automation — use `--quiet` to suppress progress output,
`-o csv` for machine-readable output, and exit codes for error handling.

```bash
#!/bin/bash
set -euo pipefail

DATE=$(date -d "yesterday" +%Y-%m-%d)
OUTPUT_DIR="./reports/${DATE}"
mkdir -p "$OUTPUT_DIR"

# Export Google Ads data
supermetrics queries execute --quiet \
  --ds-id AW \
  --fields date,campaign_name,clicks,cost,conversions \
  --start-date "$DATE" --end-date "$DATE" \
  --all \
  -o csv > "${OUTPUT_DIR}/google-ads.csv"

# Export Facebook Ads data
supermetrics queries execute --quiet \
  --ds-id FA \
  --fields date,campaign_name,spend,clicks,impressions \
  --start-date "$DATE" --end-date "$DATE" \
  --all \
  -o csv > "${OUTPUT_DIR}/facebook-ads.csv"

echo "Reports saved to ${OUTPUT_DIR}"
```

**Handle errors with exit codes:**

```bash
supermetrics queries execute \
  --ds-id GAWA --fields sessions \
  --start-date yesterday --end-date yesterday \
  -o csv > report.csv \
  || case $? in
    65) echo "Authentication failed — run: supermetrics login" ;;
    69) echo "Service unavailable — try again later" ;;
    *)  echo "Unexpected error" ;;
  esac
```

**Schedule with cron (daily at 7am):**

```
0 7 * * * /usr/local/bin/supermetrics queries execute --quiet --ds-id AW --fields date,cost,clicks --start-date yesterday --end-date yesterday -o csv >> /var/log/ads-daily.csv
```

---

### How do I use environment variables and profiles in CI/CD?

Use `SUPERMETRICS_API_KEY` for CI/CD pipelines or `--profile` for multi-tenant
setups.

**GitHub Actions example:**

```yaml
- name: Export marketing data
  env:
    SUPERMETRICS_API_KEY: ${{ secrets.SUPERMETRICS_API_KEY }}
  run: |
    supermetrics queries execute --quiet \
      --ds-id AW --fields date,cost,clicks \
      --start-date 7-days-ago --end-date yesterday \
      -o csv > google-ads-weekly.csv
```

**Multi-tenant pipeline with profiles:**

```bash
for PROFILE in client-a client-b client-c; do
  supermetrics queries execute --quiet --profile "$PROFILE" \
    --ds-id AW --fields date,campaign_name,cost \
    --start-date yesterday --end-date yesterday \
    -o csv > "${PROFILE}-google-ads.csv"
done
```

---

## Troubleshooting

### How do I debug queries that return errors?

Use `--verbose` to see the full HTTP request and response, including the API
request ID (useful for Supermetrics support).

```bash
supermetrics queries execute --verbose \
  --ds-id GAWA \
  --fields sessions \
  --start-date yesterday --end-date yesterday
```

The verbose output shows:
- Request method, URL, and headers
- Response status code, headers, and body
- API request ID for each call

**Preview a backfill or login-link request without executing:**

```bash
supermetrics backfills create --dry-run \
  --team-id 1 --transfer-id 2 \
  --range-start 2025-01-01 --range-end 2025-01-31
```

**Disable retries for faster failure:**

```bash
supermetrics queries execute --no-retry \
  --ds-id GAWA --fields sessions \
  --start-date yesterday --end-date yesterday
```

---

### How do I set custom timeouts for slow queries?

Override the default timeout with `--timeout` using Go duration format
(`30s`, `5m`, `1h`).

```bash
supermetrics queries execute \
  --ds-id AW \
  --fields campaign_name,clicks,cost \
  --start-date 2024-01-01 --end-date 2024-12-31 \
  --all \
  --timeout 10m
```

---

## Data Source ID Quick Reference

Complete list of Supermetrics data source IDs. Use these with `--ds-id` in any
`supermetrics queries execute` command.

See the full data source documentation at https://docs.supermetrics.com/apidocs/data-sources.

### Paid Advertising

| Platform | ds-id | Description |
|----------|-------|-------------|
| Google Ads | `AW` | Search, display, shopping, video campaigns |
| Facebook Ads (Meta Ads) | `FA` | Facebook and Instagram ad campaigns |
| LinkedIn Ads | `LIA` | B2B advertising on LinkedIn |
| Microsoft Advertising (Bing Ads) | `AC` | Search ads on Bing and partner networks |
| TikTok Ads | `TIK` | Short-form video advertising |
| X Ads (Twitter) | `TA` | Promoted tweets and campaigns |
| Amazon Ads | `AA` | Sponsored Products, Brands, Display |
| Amazon DSP | `ADSP` | Amazon programmatic display |
| Snapchat Marketing | `SCM` | Snap ads and filters |
| Pinterest Ads | `PIA` | Promoted pins |
| Reddit Ads | `RDA` | Reddit promoted posts |
| Quora Ads | `QA` | Quora promoted answers |
| Spotify Ads | `SPA` | Audio and display ads on Spotify |
| Google Search Ads 360 | `SA360` | Enterprise search management |
| Google Display & Video 360 | `DBM` | Programmatic display and video |
| Google Campaign Manager 360 | `DFA` | Ad serving and tracking |
| Google Ad Manager | `DFP` | Publisher ad management |
| Google AdSense | `ADM` | Publisher monetization |
| Criteo | `CRI` | Retargeting and display |
| Criteo Retail Media | `CRIRM` | Retail media advertising |
| Outbrain Amplify | `OBA` | Native content distribution |
| Taboola | `TBL` | Native advertising |
| The Trade Desk | `TTD` | Programmatic buying platform |
| AdRoll | `ADR` | Retargeting campaigns |
| Adform | `ADF` | European ad platform |
| RTB House | `RTBH` | Deep learning retargeting |
| StackAdapt | `STAC` | Programmatic native advertising |
| MNTN | `MNTN` | Connected TV advertising |
| Kwai Ads | `KWAI` | Short-video advertising in LATAM/Asia |
| LINE Ads | `LINEA` | Messaging app ads (Japan, Asia) |
| Teads | `TEADS` | Outstream video advertising |
| Xing Ads | `XING` | Professional network ads (DACH) |
| Yandex.Direct | `YAD` | Russian search advertising |
| Yahoo! Japan Search Ads | `YSA` | Search ads in Japan |
| Yahoo! Japan Display Ads | `YDA` | Display ads in Japan |
| Yahoo DSP | `VDSP` | Yahoo programmatic buying |
| IQM | `IQM` | Programmatic advertising |
| Apple Search Ads | `ASA` | App Store search ads |
| Capterra PPC | `CAP` | Software review advertising |
| Eskimi | `ESKMI` | Programmatic advertising |
| Basis | `BASIS` | Media buying platform |
| Beeswax | `BEES` | Bidder-as-a-service |
| LiveIntent | `LIVI` | Email advertising |
| Vibe | `VIBE` | Streaming TV advertising |
| Readpeak | `READP` | Native advertising |
| Flashtalking | `FLASH` | Ad serving and analytics |
| Celtra | `CELTR` | Creative management platform |
| Liftoff | `LIFT` | Mobile app marketing |
| Axon by AppLovin | `AXON` | Mobile app advertising |
| Moloco DSP | `MOLOC` | Machine learning ad platform |
| Nexxen DSP | `NEXX` | Connected TV and video DSP |
| Outbrain DSP (Zemanta) | `ZEMA` | Programmatic native |
| Google Ads Keyword Planner | `GAKEY` | Keyword research tool |
| Google Ads Account Explorer | `GAAE` | Account structure analysis |

### Web Analytics

| Platform | ds-id | Description |
|----------|-------|-------------|
| Google Analytics 4 | `GAWA` | Web and app analytics |
| Adobe Analytics 2.0 | `ADA` | Enterprise web analytics (new API) |
| Adobe Analytics | `ASC` | Enterprise web analytics (legacy) |
| Matomo | `MATO` | Privacy-focused open-source analytics |
| Google Search Console | `GW` | Search performance and indexing |
| Google Trends | `GT` | Search interest over time |
| Google PageSpeed Insights | `PSI` | Page performance metrics |
| Piwik PRO | `PIWIK` | Privacy-compliant analytics |
| Plausible | `PLAUS` | Lightweight privacy-first analytics |
| Mixpanel | `MIX` | Product analytics |
| Amplitude | `AMPLI` | Digital analytics platform |
| Hotjar | `HOT` | Heatmaps and behavior analytics |
| Piano Analytics (AT Internet) | `ATI` | European web analytics |
| Yandex.Metrica | `YAM` | Russian web analytics |
| Similarweb | `SW` | Competitive traffic analysis |
| Bing Webmaster Tools | `BW` | Bing search performance |

### Social Media (Organic)

| Platform | ds-id | Description |
|----------|-------|-------------|
| Facebook Insights | `FB` | Page-level organic metrics |
| Facebook Public Data | `FBPD` | Competitive analysis |
| Facebook Billing Data | `FBBM` | Ad account billing |
| Instagram Insights | `IGI` | Business account organic metrics |
| Instagram Public Data | `IGPD2` | Competitive benchmarking |
| YouTube | `YT2` | Channel and video analytics |
| YouTube Public Data | `YTPD` | Public video metrics |
| LinkedIn Company Pages | `LIP` | Organic page analytics |
| TikTok Organic | `TIKBA` | Account and video performance |
| X Organic (Twitter) | `TWO` | Tweet analytics |
| Pinterest Organic | `PIO2` | Pin and board performance |
| Pinterest Public Data | `PIPD` | Public pin metrics |
| Threads Insights | `THRDS` | Meta Threads analytics |
| Vimeo Public Data | `VMPD` | Video performance metrics |
| Apple Public Data | `APPD` | App Store metrics |
| Sprout Social | `SPRO` | Social media management |
| Sprinklr | `SPRIN` | Enterprise social management |
| Bambuser | `BAMBU` | Live video commerce |
| Smarp | `SMARP` | Employee advocacy |

### SEO & Content

| Platform | ds-id | Description |
|----------|-------|-------------|
| Ahrefs | `AHRF2` | Backlinks, keywords, site audit |
| Semrush Analytics | `SR` | Keyword research, competitive analysis |
| Semrush Projects | `SRP` | Position tracking, site audit |
| Meltwater | `MELT` | Media monitoring |
| Yext | `YEXT` | Local SEO and listings |

### E-commerce

| Platform | ds-id | Description |
|----------|-------|-------------|
| Shopify | `SHP` | E-commerce orders and products |
| WooCommerce | `WOOC` | WordPress e-commerce |
| BigCommerce | `BIGC` | E-commerce platform |
| Stripe | `ST` | Payment processing |
| Squarespace Commerce | `SQSP` | Website builder e-commerce |
| Amazon Seller Central | `ASELL` | Amazon marketplace |
| TikTok Shop | `TIKSH` | Social commerce |
| Shopee Commerce | `SPSL` | Southeast Asian marketplace |
| Shopee Ads | `SPAD` | Shopee advertising |
| Lazada Commerce | `LAZ` | Southeast Asian marketplace |
| Lazada Ads | `LAZAD` | Lazada advertising |
| Adobe Commerce (Magento 2) | `MAGE` | Enterprise e-commerce |
| Shopware | `SHOPW` | European e-commerce |
| PrestaShop | `PREST` | Open-source e-commerce |
| Ecwid | `ECWID` | Lightweight e-commerce |
| Wix Commerce | `WIX` | Website builder e-commerce |
| Centra | `CENTR` | Fashion e-commerce |
| Prisjakt | `PRISJ` | Price comparison |
| Recharge | `RECHA` | Subscription billing |
| Google Merchant Center | `GMC` | Product data feeds |

### Email Marketing

| Platform | ds-id | Description |
|----------|-------|-------------|
| Mailchimp | `MC` | Email campaigns and automation |
| Klaviyo | `KLAV` | E-commerce email and SMS |
| ActiveCampaign | `ACT` | Email automation |
| Campaign Monitor | `CM` | Email marketing |
| Omnisend | `OMNSD` | E-commerce marketing automation |
| Brevo (Sendinblue) | `SIB` | Email, SMS, and chat |
| Eloqua | `ELOQ` | Enterprise marketing automation |

### CRM & Sales

| Platform | ds-id | Description |
|----------|-------|-------------|
| Salesforce | `SF` | Enterprise CRM |
| HubSpot | `HS` | Inbound CRM and marketing |
| HubSpot Contacts | `HSCON` | Contact management |
| HubSpot Marketing Emails | `HSME` | Email campaign analytics |
| HubSpot Marketing Forms | `HSMF` | Form submission analytics |
| Marketo | `MARK` | B2B marketing automation |
| Pipedrive | `PIPE` | Sales pipeline management |
| Zoho CRM | `ZOHO` | Business CRM |
| Close CRM | `CLOSE` | Inside sales CRM |
| Odoo CRM | `ODOO` | Open-source ERP/CRM |
| Gong | `GONG` | Revenue intelligence |

### Mobile & Attribution

| Platform | ds-id | Description |
|----------|-------|-------------|
| AppsFlyer | `APPS` | Mobile attribution |
| Adjust | `ADJ` | Mobile measurement |
| Branch | `BRANC` | Deep linking and attribution |
| Google Play Console | `GPC` | Android app analytics |

### Data Warehouses & Storage

| Platform | ds-id | Description |
|----------|-------|-------------|
| Google BigQuery | `BQ2` | Cloud data warehouse |
| Snowflake | `SNO2` | Cloud data platform |
| Google Sheets | `GSCC2` | Spreadsheet data source |

### Affiliate & Partner Marketing

| Platform | ds-id | Description |
|----------|-------|-------------|
| CJ Affiliate | `CJA` | Affiliate network |
| Impact | `IMPA` | Partnership management |
| Awin | `AWIN` | Affiliate marketing |
| Partnerize | `PART` | Partner marketing |
| Rakuten Advertising | `RAKA` | Affiliate and display |
| Adtraction | `ADTRA` | Affiliate marketing |
| Tradedoubler | `TRD` | Affiliate network |
| Everflow | `EF` | Partner marketing platform |
| Affluent | `AFF` | Affiliate data aggregation |

### Review Platforms

| Platform | ds-id | Description |
|----------|-------|-------------|
| G2 Reviews | `WG2` | Software reviews |
| Google Play Reviews | `WGPR` | App store reviews |
| Capterra Reviews | `WCAPT` | Software comparison reviews |
| Glassdoor Reviews | `WGLAS` | Employer reviews |
| Yelp Reviews | `WYELP` | Business reviews |
| Tripadvisor Reviews | `WTRIP` | Travel reviews |
| Indeed Reviews | `WIND` | Job platform reviews |

### Verification & Measurement

| Platform | ds-id | Description |
|----------|-------|-------------|
| DoubleVerify | `DV` | Ad verification |
| Integral Ad Science | `IAS` | Media quality |
| Nielsen Digital Ad Ratings | `NIDAR` | Audience measurement |
| Quantcast | `QUCA` | Audience insights |

### Other

| Platform | ds-id | Description |
|----------|-------|-------------|
| Google My Business | `GMB` | Local business listings |
| CallRail | `CALR` | Call tracking |
| Simplesat | `SMPS` | Customer satisfaction |
| Clockify | `CLKFY` | Time tracking |
| Harvest | `HARV` | Time tracking |
| Slack | `SLACK` | Team communication |
| Ignite | `IGNT` | Social media management |
| Data Blending | `BLEND` | Cross-source data blending |

---

*For more information, see the [Supermetrics CLI README](../README.md) or run
`supermetrics --help`.*
