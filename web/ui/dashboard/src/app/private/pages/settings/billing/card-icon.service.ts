import {Injectable} from '@angular/core';
import {DomSanitizer, SafeHtml} from '@angular/platform-browser';

@Injectable({
  providedIn: 'root'
})
export class CardIconService {
  constructor(private sanitizer: DomSanitizer) {}

  getCardIconSvg(brand?: string): SafeHtml {
    if (!brand) {
      return this.sanitizer.bypassSecurityTrustHtml('');
    }

    const brandLower = brand.toLowerCase();

    if (brandLower === 'visa') {
      const svg = `<svg width="32" height="20" viewBox="0 0 262.3 85" style="enable-background:new 0 0 262.3 85;">
        <path fill="#1434CB" d="M170.9,0c-18.6,0-35.3,9.7-35.3,27.5
	c0,20.5,29.5,21.9,29.5,32.1c0,4.3-5,8.2-13.4,8.2c-12,0-21-5.4-21-5.4l-3.8,18c0,0,10.3,4.6,24.1,4.6c20.4,0,36.4-10.1,36.4-28.3
	c0-21.6-29.6-23-29.6-32.5c0-3.4,4.1-7.1,12.5-7.1c9.5,0,17.3,3.9,17.3,3.9l3.8-17.4C191.3,3.6,182.8,0,170.9,0L170.9,0z M0.5,1.3
	L0,3.9c0,0,7.8,1.4,14.9,4.3c9.1,3.3,9.7,5.2,11.3,11.1l16.7,64.3h22.4L99.6,1.3H77.3l-22.1,56l-9-47.5c-0.8-5.4-5-8.5-10.2-8.5
	C36,1.3,0.5,1.3,0.5,1.3z M108.6,1.3L91.1,83.6h21.3l17.4-82.3L108.6,1.3L108.6,1.3z M227.2,1.3c-5.1,0-7.8,2.7-9.8,7.5l-31.2,74.8
	h22.3l4.3-12.5H240l2.6,12.5h19.7L245.2,1.3L227.2,1.3L227.2,1.3z M230.1,23.6l6.6,30.9H219L230.1,23.6L230.1,23.6z"/>
      </svg>`;
      return this.sanitizer.bypassSecurityTrustHtml(svg);
    } else if (brandLower === 'mastercard') {
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
    }

    return this.sanitizer.bypassSecurityTrustHtml(''); // Default empty if no match
  }
}
