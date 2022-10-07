import { Component, EventEmitter, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrismModule } from '../prism/prism.module';
import Markdoc from '@markdoc/markdoc';
import axios from 'axios';
import { ButtonComponent } from 'src/app/components/button/button.component';

@Component({
	selector: 'sdk-documentation',
	standalone: true,
	imports: [CommonModule, PrismModule, ButtonComponent],
	templateUrl: './sdk-documentation.component.html',
	styleUrls: ['./sdk-documentation.component.scss']
})
export class SdkDocumentationComponent implements OnInit {
	@Output() onAction = new EventEmitter<any>();
	tabs = [
		{ label: 'Javascript', id: 'js' },
		{ label: 'Python', id: 'python' },
		{ label: 'PHP', id: 'php' },
		// { label: 'Ruby', id: 'ruby' },
		{ label: 'Golang', id: 'go' }
	];
	activeTab = 'js';
	activeStep: 'Installation' | 'Setup Client' | 'Create Application' | 'Add Endpoint' | 'Create Subscription' | 'Send Event' = 'Installation';
	documentation: any;
	steps: ['Installation', 'Setup Client', 'Create Application', 'Add Endpoint', 'Create Subscription', 'Send Event'] = ['Installation', 'Setup Client', 'Create Application', 'Add Endpoint', 'Create Subscription', 'Send Event'];

	constructor() {}

	ngOnInit() {
		this.switchTabs('js');
	}

	switchTabs(activeTab: string) {
		switch (activeTab) {
			case 'js':
				this.activeTab = 'js';
				this.renderDocumentation('/content/sdks/js/Installation.md');
				break;
			case 'python':
				this.activeTab = 'python';
				this.renderDocumentation('/content/sdks/python/Installation.md');
				break;
			case 'php':
				this.activeTab = 'php';
				this.renderDocumentation('/content/sdks/php/Installation.md');
				break;
			case 'ruby':
				this.activeTab = 'ruby';
				this.renderDocumentation('/content/sdks/ruby/Installation.md');
				break;
			case 'go':
				this.activeTab = 'go';
				this.renderDocumentation('/content/sdks/go/Installation.md');
				break;
			default:
				break;
		}

		this.activeStep = 'Installation';
	}

	switchStep(activeTab: 'Installation' | 'Setup Client' | 'Create Application' | 'Add Endpoint' | 'Create Subscription' | 'Send Event') {
		switch (activeTab) {
			case 'Installation':
				this.activeStep = 'Installation';
				this.renderDocumentation('/content/sdks/' + this.activeTab + '/Installation' + '.md');
				break;
			case 'Setup Client':
				this.activeStep = 'Setup Client';
				this.renderDocumentation('/content/sdks/' + this.activeTab + '/Setup Client' + '.md');
				break;
			case 'Create Application':
				this.activeStep = 'Create Application';
				this.renderDocumentation('/content/sdks/' + this.activeTab + '/Create Application' + '.md');
				break;
			case 'Add Endpoint':
				this.activeStep = 'Add Endpoint';
				this.renderDocumentation('/content/sdks/' + this.activeTab + '/Add Endpoint' + '.md');
				break;
			case 'Create Subscription':
				this.activeStep = 'Create Subscription';
				this.renderDocumentation('/content/sdks/' + this.activeTab + '/Create Subscription' + '.md');
				break;
			case 'Send Event':
				this.activeStep = 'Send Event';
				this.renderDocumentation('/content/sdks/' + this.activeTab + '/Send Event' + '.md');
				break;
			default:
				break;
		}
	}

	fetchDocumentation(mdContent: string) {
		return new Promise(async (resolve, reject) => {
			try {
				const http = await axios.create();
				const results = http.request({
					method: 'get',
					url: mdContent
				});
				resolve(results);
			} catch (error) {
				reject(error);
			}
		});
	}

	async renderDocumentation(mdContent: string) {
		const results: any = await this.fetchDocumentation(mdContent);

		const ast = Markdoc.parse(results.data);

		const content = Markdoc.transform(ast);

		this.documentation = Markdoc.renderers.html(content);
	}

	nextStep() {
		let stepIndex = this.steps.findIndex(step => step === this.activeStep) + 1;
		stepIndex === this.steps.length ? this.onAction.emit() : this.switchStep(this.steps[stepIndex]);
	}
}
