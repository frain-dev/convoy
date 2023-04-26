### Installation

Add this line to your application's Gemfile:

```console[terminal]
$ gem 'convoy'
```

And then execute:

```console[terminal]
$ bundle install
```

Or install it yourself as:

```console[terminal]
$ gem install convoy
```

### Configuration

```ruby[example]
require 'convoy'

Convoy.ssl = true
Convoy.api_key = "CO.M0aBe..."
Convoy.path_version = "v1"
Convoy.base_uri = "https://dashboard.getconvoy.io/api"
```
