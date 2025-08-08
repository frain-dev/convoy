import { Injectable } from '@angular/core';
import { Observable, from } from 'rxjs';
import { HttpService } from 'src/app/services/http/http.service';

export interface PaymentMethodUpdate {
  cardholderName: string;
  cardNumber: string;
  expiryMonth: string;
  expiryYear: string;
  cvv: string;
}

export interface BillingAddressUpdate {
  name: string;
  addressLine1: string;
  addressLine2?: string;
  city: string;
  zipCode: string;
}

export interface VatInfoUpdate {
  businessName: string;
  country: string;
  vatNumber: string;
}

@Injectable({ providedIn: 'root' })
export class BillingPaymentDetailsService {
  constructor(private httpService: HttpService) {}

  updatePaymentMethod(paymentMethod: PaymentMethodUpdate, returnFullError: boolean = false): Observable<any> {
    const orgId = this.getOrganisationId();
    return from(this.httpService.request({
      url: `/organisations/${orgId}/payment_methods`,
      method: 'post',
      body: paymentMethod,
      returnFullError: returnFullError
    }));
  }

  updateBillingAddress(billingAddress: BillingAddressUpdate): Observable<any> {
    const orgId = this.getOrganisationId();
    return from(this.httpService.request({
      url: `/organisations/${orgId}/address`,
      method: 'post',
      body: billingAddress
    }));
  }

  updateVatInfo(vatInfo: VatInfoUpdate): Observable<any> {
    const orgId = this.getOrganisationId();
    return from(this.httpService.request({
      url: `/organisations/${orgId}/tax_id`,
      method: 'post',
      body: vatInfo
    }));
  }

  private getOrganisationId(): string {
    const org = localStorage.getItem('CONVOY_ORG');
    return org ? JSON.parse(org).uid : '';
  }
} 