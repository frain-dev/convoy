import { Component, EventEmitter, Input, OnInit, Output, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CardComponent } from 'src/app/components/card/card.component';
import { ControlContainer, FormBuilder, FormGroup, FormGroupDirective, ReactiveFormsModule } from '@angular/forms';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { MonacoComponent } from '../../private/components/monaco/monaco.component';
import { PrismModule } from '../../private/components/prism/prism.module';
import { CreateSubscriptionService } from '../../private/components/create-subscription/create-subscription.service';
import { CreateSourceService } from '../../private/components/create-source/create-source.service';
import { GeneralService } from '../../services/general/general.service';
import { SelectComponent } from '../../components/select/select.component';
import {languages} from "monaco-editor";
import json = languages.json;

@Component({
	selector: 'convoy-create-portal-transform-function',
	standalone: true,
	imports: [CommonModule, CardComponent, ReactiveFormsModule, ButtonComponent, MonacoComponent, PrismModule, SelectComponent],
	providers: [{ provide: ControlContainer, useExisting: FormGroupDirective }],
	templateUrl: './create-portal-transform-function.component.html',
	styleUrls: ['./create-portal-transform-function.component.scss']
})
export class CreatePortalTransformFunctionComponent implements OnInit {
	@ViewChild('payloadEditor') payloadEditor!: MonacoComponent;
	@ViewChild('functionEditor') functionEditor!: MonacoComponent;
	@Input('transformFunction') transformFunction: any;

	@Output('updatedTransformFunction') updatedTransformFunction: EventEmitter<any> = new EventEmitter();
	tabs = ['output', 'diff'];
	activeTab = 'output';
	transformForm: FormGroup = this.formBuilder.group({
		payload: [null],
		function: [null],
		type: ['scheme.deleted']
	});
	isTransformFunctionPassed = false;
	isTestingFunction = false;
	showConsole = true;
	logs = [];
	payload: any = {
		id: 'Sample-1',
		name: 'Sample 1',
		description: 'This is sample data #1'
	};

	output: any;
	setFunction = `/* 1. While you can write multiple functions, the main function
    called for your transformation is the transform function.

2. The only argument acceptable in the transform function is
    the payload data.

3. The transform method must return a value.

4. Console logs lust be written like this
    console.log('%j', logged_item) to get printed in the log below. */

function transform(payload) {
    // Transform function here
    return payload;
}`;

	constructor(private createSubscriptionService: CreateSubscriptionService, private createSourceService: CreateSourceService, public generalService: GeneralService, private formBuilder: FormBuilder) {}

	ngOnInit(): void {}

	async testTransformFunction() {
		this.isTransformFunctionPassed = false;
		this.isTestingFunction = true;

		this.payload = this.generalService.convertStringToJson(this.payloadEditor.getValue());

		this.transformForm.patchValue({
			payload: this.payload,
			function: this.functionEditor.getValue(),
			type: 'body'
		});

		try {
			const response = await this.createSubscriptionService.testTransformFunction(this.transformForm.value);

			this.generalService.showNotification({ message: response.message, style: 'success' });

			this.output = response.data.payload;
			this.logs = response.data.log.reverse();

			if (this.logs.length > 0) this.showConsole = true;

			this.isTransformFunctionPassed = true;
			this.isTestingFunction = false;

			return this.isTransformFunctionPassed;
		} catch (error) {
			this.isTestingFunction = false;
			this.isTransformFunctionPassed = false;

			return this.isTransformFunctionPassed;
		}
	}

	log(str: string) {
		console.log(JSON.stringify({ str}))
	}

	parseLog(log: string) {
		return JSON.parse(log);
	}
}
