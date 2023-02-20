import { Component, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
	selector: 'convoy-form-loader, [convoy-form-loader]',
	standalone: true,
	imports: [CommonModule],
	template: `
		<div class="flex justify-center items-center absolute backdrop-blur-sm rounded-4px top-0 w-full h-full -ml-24px bg-white-64 bg-opacity-50 flex-col p-24px transition-all duration-300">
			<img src="/assets/img/Loader.png" class="w-110px" alt="loader icon" *ngIf="isLoading" />
			<img src="/assets/img/success.png" alt="Success gif" class="border-8 border-white-100 rounded-100px" *ngIf="!isLoading" />
		</div>
	`
})
export class FormLoaderComponent implements OnInit {
	@Input('loading') isLoading: Boolean = true;
	constructor() {}

	ngOnInit(): void {}
}
