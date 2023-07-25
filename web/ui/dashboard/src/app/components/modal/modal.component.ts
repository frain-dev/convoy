import { CommonModule } from '@angular/common';
import { Component, Directive, Input, OnInit } from '@angular/core';
import { ButtonComponent } from '../button/button.component';

// modal header
@Component({
	selector: '[convoy-modal-header]',
	imports: [CommonModule, ButtonComponent],
	standalone: true,
	template: `
		<div class="px-20px pt-20px pb-16px border-y border-y-grey-10 bg-white-100 rounded-tr-16px rounded-tl-16px w-full ">
			<div class="flex justify-between items-center max-w-[770px] m-auto">
				<ng-content></ng-content>

				<a *ngIf="fullscreen === 'true'" convoy-button fill="text" target="_blank" href="https://getconvoy.io/docs" rel="noreferrer">
					<img src="/assets/img/doc-icon-primary.svg" alt="doc icon" />
					<span class="font-medium text-14 text-primary-100 ml-2">Go to docs</span>
				</a>
			</div>
		</div>
	`
})
export class ModalHeaderComponent {
	@Input('fullscreen') fullscreen: 'true' | 'false' = 'false';
	constructor() {}
}

@Directive({
	selector: '[convoy-dialog]',
	standalone: true,
	host: { class: 'backdrop:bg-black backdrop:bg-opacity-50 p-0', '[class]': 'classes', '[id]': 'id' }
})
export class DialogDirective implements OnInit {
	@Input('position') position: 'full' | 'left' | 'right' | 'center' = 'right';
	@Input('size') size: 'sm' | 'md' | 'lg' = 'md';
	@Input('id') id!: string;
	modalSizes = { sm: 'w-[380px]', md: 'w-[460px]', lg: 'w-[600px]' };
	modalType = {
		full: ` w-full h-full`,
		left: ` ml-0 h-full`,
		right: ` mr-0 h-full`,
		center: ` rounded-[16px]`
	};
	constructor() {}

	ngOnInit(): void {}

	get classes(): string {
		return `${this.modalType[this.position]} ${this.position === 'full' ? 'bg-[#fafafe]' : 'bg-white-100 ' + this.modalSizes[this.size]}`;
	}
}
