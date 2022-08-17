import { Story, Meta } from '@storybook/angular/types-6-0';
import { moduleMetadata } from '@storybook/angular';
import { RadioComponent } from '../../app/components/radio/radio.component';
import { FormControl, FormGroup, ReactiveFormsModule } from '@angular/forms';

export default {
	title: 'Example/Radio',
	component: RadioComponent,
	decorators: [
		moduleMetadata({
			imports: [ReactiveFormsModule]
		})
	],
	argTypes: {
		label: {
			control: { type: 'text' }
		},
		description: {
			control: { type: 'text' }
		},
		checked: {
			control: { type: 'boolean' }
		}
	}
} as Meta;

const Template: Story<RadioComponent> = (args: RadioComponent) => {
	const formGroup = new FormGroup({
		enableForm: new FormControl(undefined)
	});

	return {
		component: RadioComponent,
		template: `
        <form [formGroup]="form">
          <convoy-radio
            [label]="label"
            [description]="description"
            [checked]="checked"
            formControlName="enableForm"
          >
          </convoy-radio>
        </form>
      `,
		props: {
			...args,
			form: formGroup
		}
	};
};

export const Base = Template.bind({});
Base.args = {
	label: 'Convoy Radio Label',
	description: 'Convoy Radio Description',
	checked: false
} as Partial<RadioComponent>;
