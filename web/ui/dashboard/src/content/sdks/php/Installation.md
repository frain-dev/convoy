### Installation

To install the package, you will need to be using Composer in your project.

To get started quickly,

```bash[terminal]
$ composer require frain/convoy symfony/http-client nyholm/psr7
```

### Configuration

Next, import the `convoy` module and setup with your auth credentials.

```php[example]
use Convoy\Convoy;

$convoy = new Convoy(["api_key" => "your_api_key"]);
```
