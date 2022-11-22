import { CommonModule } from '@angular/common';
import { Component, EventEmitter, forwardRef, Input, OnInit, Output } from '@angular/core';
import { ControlContainer, ControlValueAccessor, NG_VALUE_ACCESSOR, ReactiveFormsModule } from '@angular/forms';

@Component({
	selector: 'convoy-toggle',
	standalone: true,
	imports: [CommonModule, ReactiveFormsModule],
	templateUrl: './toggle.component.html',
	styleUrls: ['./toggle.component.scss'],
	providers: [
		{
			provide: NG_VALUE_ACCESSOR,
			useExisting: forwardRef(() => ToggleComponent),
			multi: true
		}
	]
})
export class ToggleComponent implements OnInit, ControlValueAccessor {
	@Input('isChecked') isChecked = false;
	@Input('label') label!: string;
	@Input('name') id!: string;
	@Input('className') class!: string;
	@Input('formControlName') formControlName?: string;
	@Output('onChange') onChange = new EventEmitter<any>();
	control!: any;

	constructor(private controlContainer: ControlContainer) {}

	ngOnInit(): void {
		if (this.formControlName) {
			if (this.controlContainer.control?.get(this.formControlName)) this.control = this.controlContainer.control.get(this.formControlName);
		}
	}

	registerOnChange() {}

	registerOnTouched() {}

	writeValue() {}

	setDisabledState() {}
}
