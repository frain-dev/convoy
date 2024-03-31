import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TooltipComponent } from '../tooltip/tooltip.component';
import { GeneralService } from 'src/app/services/general/general.service';

@Component({
	selector: 'convoy-multi-input',
	standalone: true,
	imports: [CommonModule, TooltipComponent],
	templateUrl: './multi-input.component.html',
	styleUrls: ['./multi-input.component.scss']
})
export class MultiInputComponent implements OnInit {
	@Output() inputValues = new EventEmitter<string[]>();
	@Input('prefilledKeys') prefilledKeys?: string[];
	@Input('label') label?: string;
	@Input('tooltip') tooltip?: string;
	@Input('required') required: 'true' | 'false' = 'false';
	@Input('action') action!: 'view' | 'create' | 'update';

	keys: string[] = [];

	constructor(private generalService: GeneralService) {}

	ngOnInit(): void {
		if (this.prefilledKeys?.length) this.keys = this.prefilledKeys;
	}

	addKey() {
		const inputField = document.getElementById('input');
		const inputValue = document.getElementById('input') as HTMLInputElement;

		const commaSeparatedValues = inputValue.value.split(/\s*,\s*/);

		if (commaSeparatedValues && commaSeparatedValues.length > 1) {
			commaSeparatedValues.forEach(separateValue => {
				if (!this.keys?.includes(separateValue)) this.keys.push(separateValue);
			});
			inputValue.value = '';
			this.keys = this.keys.filter(e => String(e).trim());
			this.inputValues.emit(this.keys);
		}

		inputField?.addEventListener('keydown', e => {
			const key = e.keyCode || e.charCode;
			if (key == 8) {
				e.stopImmediatePropagation();
				if (this.keys?.length > 0 && !inputValue?.value) this.keys.splice(-1);
			}
			if (e.which === 188 || e.key == ' ') {
				if (!this.keys?.includes(inputValue?.value)) this.keys.push(inputValue?.value);
				else this.generalService.showNotification({ message: 'You have entered this key previously', style: 'warning' });
				inputValue.value = '';
				this.keys = this.keys.filter(e => String(e).trim());
				this.inputValues.emit(this.keys);
				e.preventDefault();
			}
		});
	}

	removeKey(key: string) {
		this.keys = this.keys.filter(e => e !== key);
		this.inputValues.emit(this.keys);
	}

	focusInput() {
		document.getElementById('input')?.focus();
	}
}
