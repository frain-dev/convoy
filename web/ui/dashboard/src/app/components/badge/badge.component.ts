import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, Component, Input, OnInit } from '@angular/core';

@Component({
	selector: 'convoy-badge',
	standalone: true,
	imports: [CommonModule],
	templateUrl: './badge.component.html',
	styleUrls: ['./badge.component.scss'],
	changeDetection: ChangeDetectionStrategy.OnPush
})
export class BadgeComponent implements OnInit {
	@Input('texture') texture: 'dark' | 'light' = 'light';
	@Input('text') text!: string;
	@Input('className') class!: string;
	@Input('show-text') showText = true;

	constructor() {}

	ngOnInit(): void {}

	get firstletters(): string {
		const firstLetters = this.text
			.split(' ')
			.map(word => word[0])
			.join('')
			.slice(0, 2);
		return firstLetters;
	}
}
