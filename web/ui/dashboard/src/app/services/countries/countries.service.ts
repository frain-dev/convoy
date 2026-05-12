import {Injectable} from '@angular/core';
import { HttpClient } from '@angular/common/http';
import {Observable} from 'rxjs';
import {map} from 'rxjs/operators';

export interface PopulationCount {
  year: string;
  value: string;
  sex: string;
  reliabilty: string;
}

export interface CityData {
  city: string;
  country: string;
  populationCounts: PopulationCount[];
}

export interface CountriesResponse {
  error: boolean;
  msg: string;
  data: CityData[];
}

interface CountryCitiesResponse {
  error: boolean;
  msg: string;
  data: string[];
}

interface CountryStatesResponse {
  error: boolean;
  msg: string;
  data?: {
    states?: Array<{
      name: string;
      state_code: string;
    }>;
  };
}

interface StateCitiesResponse {
  error: boolean;
  msg: string;
  data: string[];
}

interface CountriesIsoResponse {
  error: boolean;
  msg: string;
  data: { name: string; Iso2: string; Iso3: string }[];
}

@Injectable({
  providedIn: 'root'
})
export class CountriesService {
  private readonly API_BASE_URL = 'https://countriesnow.space/api/v0.1';

  constructor(private http: HttpClient) {}

  /**
   * Fetch all cities with population data from the API
   */
  getCitiesData(): Observable<CountriesResponse> {
    return this.http.get<CountriesResponse>(`${this.API_BASE_URL}/countries/population/cities`);
  }

  /**
   * Get all unique countries with ISO codes
   */
  getCountries(): Observable<{ code: string; name: string }[]> {
    return this.http.get<CountriesIsoResponse>(`${this.API_BASE_URL}/countries/iso`).pipe(
      map(response => {
        if (response.error || !response.data) {
          return [];
        }

        return response.data
          .filter(c => !!c.name && !!c.Iso2)
          .map(c => ({ code: c.Iso2, name: c.name }))
          .sort((a, b) => a.name.localeCompare(b.name));
      })
    );
  }

  /**
   * Get cities for a specific country
   */
  getCitiesForCountry(countryName: string): Observable<string[]> {
    return this.http.get<CountryCitiesResponse>(`${this.API_BASE_URL}/countries/cities/q?country=${encodeURIComponent(countryName)}`).pipe(
      map(response => {
        if (response.error || !Array.isArray(response.data)) {
          return [];
        }

        return [...new Set(response.data)].sort();
      })
    );
  }

  getStatesForCountry(countryName: string): Observable<string[]> {
    return this.http.get<CountryStatesResponse>(`${this.API_BASE_URL}/countries/states/q?country=${encodeURIComponent(countryName)}`).pipe(
      map(response => {
        const states = response.data?.states || [];
        if (response.error || !Array.isArray(states)) {
          return [];
        }

        return [...new Set(states.map(state => state.name).filter(Boolean))].sort();
      })
    );
  }

  getCitiesForCountryAndState(countryName: string, stateName: string): Observable<string[]> {
    const country = encodeURIComponent(countryName);
    const state = encodeURIComponent(stateName);

    return this.http.get<StateCitiesResponse>(`${this.API_BASE_URL}/countries/state/cities/q?country=${country}&state=${state}`).pipe(
      map(response => {
        if (response.error || !Array.isArray(response.data)) {
          return [];
        }

        return [...new Set(response.data)].sort();
      })
    );
  }
}
