const ISO_URL = 'https://countriesnow.space/api/v0.1/countries/iso';
const CITIES_URL = 'https://countriesnow.space/api/v0.1/countries/cities/q?country=';
const CONCURRENCY = 10;

async function fetchJson(url) {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`HTTP ${res.status} for ${url}`);
  return res.json();
}

async function checkCountry(country) {
  const url = `${CITIES_URL}${encodeURIComponent(country)}`;
  try {
    const response = await fetchJson(url);
    const count = Array.isArray(response.data) ? response.data.length : 0;
    return {
      country,
      ok: !response.error && count > 0,
      count,
      msg: response.msg || ''
    };
  } catch (error) {
    return {
      country,
      ok: false,
      count: 0,
      msg: 'request_failed'
    };
  }
}

async function runWithConcurrency(items, worker, concurrency) {
  const results = [];
  let index = 0;

  async function runner() {
    while (index < items.length) {
      const current = index++;
      results[current] = await worker(items[current]);
    }
  }

  await Promise.all(Array.from({ length: Math.min(concurrency, items.length) }, () => runner()));
  return results;
}

async function main() {
  const isoResponse = await fetchJson(ISO_URL);
  const countries = (isoResponse.data || []).map(c => c.name).filter(Boolean);

  const results = await runWithConcurrency(countries, checkCountry, CONCURRENCY);
  const failures = results.filter(r => !r.ok);
  const successes = results.filter(r => r.ok);

  console.log(`Checked countries: ${results.length}`);
  console.log(`With cities: ${successes.length}`);
  console.log(`Empty/failed: ${failures.length}`);

  if (failures.length > 0) {
    console.log('\nCountries with empty/failed city response:');
    failures.forEach(f => {
      console.log(`- ${f.country} (${f.msg})`);
    });
  }

  if (failures.length > 0) {
    process.exitCode = 1;
  }
}

await main();
