import {Component, OnInit} from '@angular/core';
import {CityData, CountriesResponse, CountriesService} from 'src/app/services/countries/countries.service';

@Component({
  selector: 'app-api-test',
  template: `
    <div style="padding: 20px;">
      <h2>CountriesNow API Test</h2>

      <button (click)="fetchData()" [disabled]="isLoading">
        {{ isLoading ? 'Loading...' : 'Fetch API Data' }}
      </button>

      <div *ngIf="error" style="color: red; margin-top: 10px;">
        Error: {{ error }}
      </div>

      <div *ngIf="apiResponse" style="margin-top: 20px;">
        <h3>API Response Summary:</h3>
        <ul>
          <li><strong>Error:</strong> {{ apiResponse.error }}</li>
          <li><strong>Message:</strong> {{ apiResponse.msg }}</li>
          <li><strong>Total Cities:</strong> {{ apiResponse.data?.length || 0 }}</li>
        </ul>

        <h3>Unique Countries ({{ uniqueCountries.length }}):</h3>
        <div style="max-height: 300px; overflow-y: auto; border: 1px solid #ccc; padding: 10px;">
          <div *ngFor="let country of uniqueCountries">{{ country }}</div>
        </div>

        <h3>Sample City Data:</h3>
        <div *ngIf="sampleCity" style="border: 1px solid #ccc; padding: 10px; margin-top: 10px;">
          <pre>{{ sampleCity | json }}</pre>
        </div>

        <h3>Countries with Most Cities:</h3>
        <div style="max-height: 200px; overflow-y: auto; border: 1px solid #ccc; padding: 10px;">
          <div *ngFor="let item of countriesWithCityCounts.slice(0, 20)">
            {{ item.country }}: {{ item.cityCount }} cities
          </div>
        </div>
      </div>
    </div>
  `,
  styles: []
})
export class ApiTestComponent implements OnInit {
  apiResponse: CountriesResponse | null = null;
  uniqueCountries: string[] = [];
  sampleCity: CityData | null = null;
  countriesWithCityCounts: { country: string; cityCount: number }[] = [];
  isLoading = false;
  error: string | null = null;

  constructor(private countriesService: CountriesService) {}

  ngOnInit() {
    this.fetchData();
  }

  fetchData() {
    this.isLoading = true;
    this.error = null;

    this.countriesService.getCitiesData().subscribe({
      next: (response) => {
        this.apiResponse = response;
        this.analyzeResponse(response);
        this.isLoading = false;
      },
      error: (error) => {
        this.error = error.message || 'Failed to fetch data';
        this.isLoading = false;
        console.error('API Error:', error);
      }
    });
  }

  private analyzeResponse(response: CountriesResponse) {
    if (!response.data) return;

    // Get unique countries
    const countrySet = new Set<string>();
    response.data.forEach(city => countrySet.add(city.country));
    this.uniqueCountries = Array.from(countrySet).sort();

    // Get sample city
    this.sampleCity = response.data[0];

    // Count cities per country
    const countryCounts = new Map<string, number>();
    response.data.forEach(city => {
      const count = countryCounts.get(city.country) || 0;
      countryCounts.set(city.country, count + 1);
    });

    this.countriesWithCityCounts = Array.from(countryCounts.entries())
      .map(([country, cityCount]) => ({ country, cityCount }))
      .sort((a, b) => b.cityCount - a.cityCount);

    console.log('Analysis complete:', {
      totalCities: response.data.length,
      uniqueCountries: this.uniqueCountries.length,
      countriesWithCityCounts: this.countriesWithCityCounts.slice(0, 10)
    });
  }
}
