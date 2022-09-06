import { FormControl, FormGroup, ReactiveFormsModule } from '@angular/forms';
import { Story, Meta } from '@storybook/angular/types-6-0';
import { moduleMetadata } from '@storybook/angular';
import { ToggleComponent } from '../../app/components/toggle/toggle.component';

export default {
	title: 'Example/Toggle',
	component: ToggleComponent,
    decorators: [
		moduleMetadata({
			imports: [ReactiveFormsModule]
		})
	],
	argTypes: {
		isChecked: {
			control: { type: 'boolean' }
		}
	}
} as Meta;

const Template: Story<ToggleComponent> = (args: ToggleComponent) => {
	const formGroup = new FormGroup({
		toggleForm: new FormControl(undefined)
	});

	return {
		component: ToggleComponent,
		template: `
        <form [formGroup]="form">
          <convoy-toggle
            [isChecked]="isChecked"
            formControlName="toggleForm"
          >
          </convoy-toggle>
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
	isChecked: false,
};
