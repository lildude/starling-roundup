# starling-roundup

[![Build Status](https://travis-ci.org/lildude/starling-roundup.svg?branch=master)](https://travis-ci.org/lildude/starling-roundup) [![Coverage Status](https://coveralls.io/repos/github/lildude/starling-roundup/badge.svg?branch=master)](https://coveralls.io/github/lildude/starling-roundup?branch=master)

This application allows you to round-up your Starling bank transactions to the nearest £1 and transfer the delta to a savings goal.

Note: Starling now has this functionality natively. Don't set `STARLING_SAVING_GOAL` if you want to use Starling's native functionality. If you set this and have Starling's setting enabled, you will end up with duplicate transfers.

It can also "sweep" the balance in your account as the time of receiving an inbound faster payment or Nostro deposit to a savings goal.

This implementation is a fork of the original work at https://github.com/billglover/starling-roundup, but targeted at Heroku. Why Heroku? Because I already use Heroku, it has a simple "click" deploy method and gives me all the web server resources and functionality I need without having to string together, and individually pay for, a ton of AWS services. This runs quite happily in the free micro dyno.

## How it works

1. Starling Bank triggers a webhook on each transaction.
2. This webhook is configured to POST the transaction data to this application running on Heroku.
3. The application checks the signature of the request, checks the transaction UID and if it's not the transaction we rounded up, rounds up the value, and sends a request back to Starling Bank to move the delta to a savings goal.

## Installation

### Pre-Requisites

- A [Starling Bank](https://starlingbank.com) account
- A [Starling Bank Developer](https://developer.starlingbank.com) account
- A [Heroku](https://heroku.com) account

### Configuring Your App

- Deploy the application to Heroku: [Snazzy button coming :soon:]
- Take note of the application URL, this is the webhook URL you'll need to enter on Starling.
- Register an application with your Starling developer account.
- Create a personal webhook using the URL returned above.
- Make a note of the webhook secret and the personal access token.
- Set the following configuration variables, either in the Heroku UI, or using the Heroku CLI:
  - `STARLING_WEBHOOK_SECRET` - used to validate inbound requests
  - `STARLING_PERSONAL_ACCESS_TOKEN` - used to request transfers to savings goal
  - `STARLING_SAVING_GOAL` -  the identifier of the target savings goal. If not set, rounding will not occur.
  - `STARLING_SWEEP_THRESHOLD` - optional threshold, integer in pence, for incoming payments to trigger a sweep. If not set, sweeping will not occur.
  - `STARTLING_SWEEP_SAVING_GOAL` - optional identifier of the target savings goal for sweeps. Defaults to `STARLING_SAVING_GOAL` if not set.

  For example, from the CLI:
  ```
  $ heroku config:set STARLING_WEBHOOK_SECRET="your-secret" STARLING_PERSONAL_ACCESS_TOKEN="your-personal-access-token" STARLING_SAVING_GOAL="your-savings-goal-id"
  ```

### Local Development and Testing

- Save your Heroku config vars to a `.env` file: `heroku config:get -s  >.env`. Don't commit this file to your repo unless you really don't like your money.
- Start the application: `heroku local`.
- Send test requests to 0.0.0.0:5000 using something like curl or httpie.

### Contributing

Issues and pull requests are both welcome.

## Similar Projects

- [Starling Roundup (for AWS)](https://github.com/billglover/starling-roundup) - the origin of this fork.
- [Starling CoinJar](https://github.com/cooperaj/starling-coinjar)
