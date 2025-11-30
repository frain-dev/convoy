import { Component, OnInit, OnDestroy } from '@angular/core';

interface Testimonial {
	name: string;
	role: string;
	text: string;
}

@Component({
  selector: 'convoy-public',
  templateUrl: './public.component.html',
  styleUrls: ['./public.component.scss']
})
export class PublicComponent implements OnInit, OnDestroy {
	private readonly testimonials: Testimonial[] = [
		{
			name: 'Manan Patel',
			role: 'CTO at Neynar, Ex Coinbase',
			text: 'We tried a few different solutions in the market, but Convoy stood out for its dynamic filtering capabilities, and it was extremely easy to set up; we had test webhooks sent within the hour.'
		},
		{
			name: 'Michael Raines',
			role: 'Principal Engineer at Spruce Health, Ex AWS',
			text: 'We considered building a webhooks system internally but quickly realised that reaching the quality and robustness our customers deserve would be highly time-consuming. Convoy offered this out-of-the-box.'
		},
		{
			name: 'Aravindkumar Rajendiran',
			role: 'Co-Founder and CTO at Maple Billing',
			text: 'Convoy had everything (retries, signatures, SDKs) we needed for a webhook gateway. We were able to get it up and running within a few hours instead of months. It allowed our engineering team to focus on building our core product.'
		},
		{
			name: 'Pascal Delange',
			role: 'CTO at Marble, Ex-Director of Engineering, Shine',
			text: 'We appreciate that they handle all the complexity of webhooks retries and dispatching for us, letting us focus on our core business.'
		}
	];

	currentTestimonialIndex = 0;
	private rotationInterval?: number;

	constructor() { }

	ngOnInit(): void {
		this.startRotation();
	}

	ngOnDestroy(): void {
		this.stopRotation();
	}

	get currentTestimonial(): Testimonial {
		return this.testimonials[this.currentTestimonialIndex];
	}

	private startRotation(): void {
		// Rotate every 15 seconds
		this.rotationInterval = window.setInterval(() => {
			this.rotateToNext();
		}, 15000);
	}

	private stopRotation(): void {
		if (this.rotationInterval) {
			clearInterval(this.rotationInterval);
			this.rotationInterval = undefined;
		}
	}

	private rotateToNext(): void {
		this.currentTestimonialIndex = (this.currentTestimonialIndex + 1) % this.testimonials.length;
	}

	// Public method to manually rotate (for future UI controls)
	rotateTo(index: number): void {
		if (index >= 0 && index < this.testimonials.length) {
			this.currentTestimonialIndex = index;
		}
	}
}
