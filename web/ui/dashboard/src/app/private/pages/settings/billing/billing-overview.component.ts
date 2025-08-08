import { Component } from '@angular/core';
import { DomSanitizer, SafeHtml } from '@angular/platform-browser';

@Component({
  selector: 'app-billing-overview',
  templateUrl: './billing-overview.component.html',
  styleUrls: ['./billing-overview.component.scss']
})
export class BillingOverviewComponent {
  cardNumber: string = '5105105105105100'; // Mastercard test card
  maskedCardNumber: string = '5100'; // Last 4 digits
  
  constructor(private sanitizer: DomSanitizer) {}
  
  getCardIconSvg(): SafeHtml {
    // Detect card type by number pattern
    const cleanNumber = this.cardNumber.replace(/[\s-]/g, '');
    
    if (/^5[1-5]/.test(cleanNumber)) {
      // Mastercard
      const svg = `<svg width="32" height="20" viewBox="0 0 116.5 72" style="enable-background:new 0 0 116.5 72;">
        <g>
          <g>
            <rect x="42.5" y="7.7" fill="#FF5F00" width="31.5" height="56.6"></rect>
            <path fill="#EB001B" d="M44.5,36c0-11,5.1-21.5,13.7-28.3C42.6-4.6,20-1.9,7.7,13.8C-4.6,29.4-1.9,52,13.8,64.3
              c13.1,10.3,31.4,10.3,44.5,0C49.6,57.5,44.5,47,44.5,36z"></path>
            <path fill="#F79E1B" d="M116.5,36c0,19.9-16.1,36-36,36c-8.1,0-15.9-2.7-22.2-7.7c15.6-12.3,18.3-34.9,6-50.6c-1.8-2.2-3.8-4.3-6-6
              c15.6-12.3,38.3-9.6,50.5,6.1C113.8,20.1,116.5,27.9,116.5,36z"></path>
            <path fill="#F79E1B" d="M113.1,58.3v-1.2h0.5v-0.2h-1.2v0.2h0.5v1.2H113.1z M115.4,58.3v-1.4H115l-0.4,1l-0.4-1h-0.4v1.4h0.3v-1.1
              l0.4,0.9h0.3l0.4-0.9v1.1H115.4z"></path>
          </g>
        </g>
      </svg>`;
      return this.sanitizer.bypassSecurityTrustHtml(svg);
    } else if (/^4/.test(cleanNumber)) {
      // Visa
      const svg = `<svg width="32" height="20" viewBox="0 0 48 16">
        <path fill="#1A1F71" d="M45.5,0H2.5C1.1,0,0,1.1,0,2.5v11C0,14.9,1.1,16,2.5,16h43C46.9,16,48,14.9,48,13.5v-11C48,1.1,46.9,0,45.5,0z"/>
        <text x="24" y="11" text-anchor="middle" fill="#FFFFFF" font-family="Arial" font-size="8" font-weight="bold">VISA</text>
      </svg>`;
      return this.sanitizer.bypassSecurityTrustHtml(svg);
    } else if (/^3[47]/.test(cleanNumber)) {
      // American Express
      const svg = `<svg width="32" height="20" viewBox="0 0 48 16">
        <path fill="#006FCF" d="M45.5,0H2.5C1.1,0,0,1.1,0,2.5v11C0,14.9,1.1,16,2.5,16h43C46.9,16,48,14.9,48,13.5v-11C48,1.1,46.9,0,45.5,0z"/>
        <text x="24" y="11" text-anchor="middle" fill="#FFFFFF" font-family="Arial" font-size="6" font-weight="bold">AMEX</text>
      </svg>`;
      return this.sanitizer.bypassSecurityTrustHtml(svg);
    } else if (/^6(?:011|5)/.test(cleanNumber)) {
      // Discover
      const svg = `<svg width="32" height="20" viewBox="0 0 48 16">
        <path fill="#FF6600" d="M45.5,0H2.5C1.1,0,0,1.1,0,2.5v11C0,14.9,1.1,16,2.5,16h43C46.9,16,48,14.9,48,13.5v-11C48,1.1,46.9,0,45.5,0z"/>
        <text x="24" y="11" text-anchor="middle" fill="#FFFFFF" font-family="Arial" font-size="6" font-weight="bold">DISCOVER</text>
      </svg>`;
      return this.sanitizer.bypassSecurityTrustHtml(svg);
    }
    
    return this.sanitizer.bypassSecurityTrustHtml(''); // Default empty if no match
  }
} 