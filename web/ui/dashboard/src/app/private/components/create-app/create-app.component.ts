import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { FormArray, FormBuilder, FormGroup, Validators } from '@angular/forms';

@Component({
	selector: 'app-create-app',
	templateUrl: './create-app.component.html',
	styleUrls: ['./create-app.component.scss']
})
export class CreateAppComponent implements OnInit {
	@Input() editAppMode: boolean = false;
	@Output() discardCreateApp = new EventEmitter<any>();
	eventTags!: string[];
	isCreatingNewApp: boolean = false;
	addNewAppForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required],
		support_email: [''],
		secret: [''],
		description: [''],
		config: [''],
		is_disabled: [false],
		endpoints: this.formBuilder.array([])
	});
	constructor(private formBuilder: FormBuilder) {}

	ngOnInit(): void {}

	get endpoints(): FormArray {
		return this.addNewAppForm.get('endpoints') as FormArray;
	}

	getSingleEndpoint(index: any) {
		return ((this.addNewAppForm.get('endpoints') as FormArray)?.controls[index] as FormGroup)?.controls;
	}

	newEndpoint(): FormGroup {
		return this.formBuilder.group({
			url: ['', Validators.required],
			events: [''],
			tag: ['', Validators.required],
			description: ['', Validators.required]
		});
	}

	addEndpoint() {
		this.endpoints.push(this.newEndpoint());
	}

	removeEndpoint(i: number) {
		this.endpoints.removeAt(i);
	}

	removeEventTag(tag: string) {
		this.eventTags = this.eventTags.filter(e => e !== tag);
	}

	addTag() {
		const addTagInput = document.getElementById('tagInput');
		const addTagInputValue = document.getElementById('tagInput') as HTMLInputElement;
		addTagInput?.addEventListener('keydown', e => {
			if (e.which === 188) {
				if (this.eventTags.includes(addTagInputValue?.value)) {
					addTagInputValue.value = '';
					this.eventTags = this.eventTags.filter(e => String(e).trim());
				} else {
					this.eventTags.push(addTagInputValue?.value);
					addTagInputValue.value = '';
					this.eventTags = this.eventTags.filter(e => String(e).trim());
				}
				e.preventDefault();
			}
		});
	}

	createNewApp() {}

	closeCreateAppInstance() {
		this.discardCreateApp.emit();
	}
}
