import {Injectable} from '@angular/core';
import {from, Observable, of} from 'rxjs';
import {catchError, map, mergeMap} from 'rxjs/operators';
import {HttpService} from 'src/app/services/http/http.service';

export interface PaymentMethodUpdate {
  cardholderName: string;
  cardNumber: string;
  expiryMonth: string;
  expiryYear: string;
  cvv: string;
}

export interface PaymentMethodDetails {
  cardholderName: string;
  last4: string;
  brand: string;
  expiryMonth: string;
  expiryYear: string;
}

export interface BillingAddressUpdate {
  name: string;
  addressLine1: string;
  addressLine2?: string;
  country: string;
  city: string;
  zipCode: string;
}

export interface BillingAddressDetails {
  name: string;
  addressLine1: string;
  addressLine2?: string;
  country: string;
  city: string;
  zipCode: string;
}

export interface VatInfoUpdate {
  businessName: string;
  country: string;
  vatNumber: string;
}

export interface VatInfoDetails {
  businessName: string;
  country: string;
  vatNumber: string;
}

@Injectable({ providedIn: 'root' })
export class BillingPaymentDetailsService {
  constructor(private httpService: HttpService) {}

  // Get billing configuration including payment provider details
  getBillingConfig(): Observable<any> {
    return from(this.httpService.request({
      url: '/billing/config',
      method: 'get'
    })).pipe(
      catchError((error) => {
        console.error('Failed to fetch billing configuration:', error);
        throw error;
      })
    );
  }


  // Get internal organisation ID from Overwatch
  getInternalOrganisationId(externalOrgId: string): Observable<any> {
    return from(this.httpService.request({
      url: `/billing/organisations/${externalOrgId}/internal_id`,
      method: 'get'
    })).pipe(
      catchError((error) => {
        console.error('Failed to fetch internal organisation ID:', error);
        throw error;
      })
    );
  }

  // Get existing payment details - this endpoint exists
  getPaymentMethodDetails(): Observable<PaymentMethodDetails> {
    const orgId = this.getOrganisationId();
    return from(this.httpService.request({
      url: `/billing/organisations/${orgId}/payment_methods`,
      method: 'get'
    })).pipe(
      map((response: any) => {
        // The API returns an array, get the first/default payment method
        const paymentMethods = response.data || [];
        if (paymentMethods.length > 0) {
          const pm = paymentMethods[0]; // Get the first/default payment method
          return {
            cardholderName: pm.cardholder_name || 'Cardholder Name',
            last4: pm.last4 || '0000',
            brand: pm.brand || 'unknown',
            expiryMonth: pm.exp_month?.toString() || '',
            expiryYear: pm.exp_year?.toString() || ''
          };
        }
        return {
          cardholderName: '',
          last4: '',
          brand: '',
          expiryMonth: '',
          expiryYear: ''
        };
      }),
      catchError((error) => {
        console.error('Failed to fetch payment method details:', error);
        return of({
          cardholderName: '',
          last4: '',
          brand: '',
          expiryMonth: '',
          expiryYear: ''
        });
      })
    );
  }

  // Get existing billing address - get from organisation details
  getBillingAddress(): Observable<BillingAddressDetails> {
    const orgId = this.getOrganisationId();
    return from(this.httpService.request({
      url: `/organisations/${orgId}`,
      method: 'get'
    })).pipe(
      map((response: any) => {
        const org = response.data || {};
        return {
          name: org.billing_name || '', // Use billing_name only, no fallback to avoid confusion with VAT
          addressLine1: org.billing_address || '',
          addressLine2: org.billing_address_line2 || '', // Use proper billing_address_line2 field (handles null)
          country: org.billing_country || '',
          city: org.billing_city || '',
          zipCode: org.billing_zip || ''
        };
      }),
      catchError((error) => {
        console.error('Failed to fetch billing address:', error);
        return of({
          name: '',
          addressLine1: '',
          addressLine2: '',
          country: '',
          city: '',
          zipCode: ''
        });
      })
    );
  }

  // Get existing VAT information - get from organisation details
  getVatInfo(): Observable<VatInfoDetails> {
    const orgId = this.getOrganisationId();
    return from(this.httpService.request({
      url: `/organisations/${orgId}`,
      method: 'get'
    })).pipe(
      map((response: any) => {
        const org = response.data || {};
        return {
          businessName: org.name || 'Business Name', // Use org name as default, but should be separate
          country: org.billing_country || '',
          vatNumber: org.tax_number || ''
        };
      }),
      catchError((error) => {
        console.error('Failed to fetch VAT info:', error);
        return of({
          businessName: '',
          country: '',
          vatNumber: ''
        });
      })
    );
  }

  getSetupIntent(): Observable<any> {
    const orgId = this.getOrganisationId();
    return from(this.httpService.request({
      url: `/billing/organisations/${orgId}/payment_methods/setup_intent`,
      method: 'get'
    }));
  }

  updatePaymentMethod(paymentMethod: PaymentMethodUpdate, returnFullError: boolean = false): Observable<any> {
    const orgId = this.getOrganisationId();
    return from(this.httpService.request({
      url: `/billing/organisations/${orgId}/payment_methods`,
      method: 'post',
      body: paymentMethod,
      returnFullError: returnFullError
    }));
  }

  updateBillingAddress(billingAddress: BillingAddressUpdate): Observable<any> {
    const orgId = this.getOrganisationId();

    // Update billing address with billing_name field
    const addressData = {
      billing_name: billingAddress.name,
      billing_address: billingAddress.addressLine1,
      billing_address_line2: billingAddress.addressLine2 || '', // Use proper address line 2 field
      billing_city: billingAddress.city,
      billing_state: '', // Keep state separate from address line 2
      billing_zip: billingAddress.zipCode,
      billing_country: billingAddress.country // Use the selected country from the form
    };

    return from(this.httpService.request({
      url: `/billing/organisations/${orgId}/address`,
      method: 'post',
      body: addressData
    }));
  }

  updateVatInfo(vatInfo: VatInfoUpdate): Observable<any> {
    const orgId = this.getOrganisationId();

    // Update the organization name for the VAT business name
    const orgUpdateData = {
      name: vatInfo.businessName
    };

    // Update tax ID information
    const taxData = {
      tax_id_type: 'VAT',
      tax_number: vatInfo.vatNumber
    };

    // Update billing country for VAT
    const addressData = {
      billing_country: vatInfo.country
    };

    // Update organization name first, then tax ID, then country
    return from(this.httpService.request({
      url: `/organisations/${orgId}`,
      method: 'put',
      body: orgUpdateData
    })).pipe(
      mergeMap(() => {
        return from(this.httpService.request({
          url: `/billing/organisations/${orgId}/tax_id`,
          method: 'post',
          body: taxData
        }));
      }),
      mergeMap(() => {
        return from(this.httpService.request({
          url: `/billing/organisations/${orgId}/address`,
          method: 'post',
          body: addressData
        }));
      })
    );
  }

  private getOrganisationId(): string {
    const org = localStorage.getItem('CONVOY_ORG');
    console.log('Raw org from localStorage:', org);
    
    if (!org) {
      console.error('No organisation found in localStorage');
      throw new Error('No organisation found. Please refresh the page and try again.');
    }
    
    try {
      const orgData = JSON.parse(org);
      console.log('Parsed org data:', orgData);
      
      if (!orgData.uid) {
        console.error('No organisation UID found in localStorage data:', orgData);
        throw new Error('Invalid organisation data. Please refresh the page and try again.');
      }
      
      console.log('Using organisation ID:', orgData.uid);
      return orgData.uid;
    } catch (error) {
      console.error('Error parsing organisation data from localStorage:', error);
      throw new Error('Invalid organisation data. Please refresh the page and try again.');
    }
  }
}
