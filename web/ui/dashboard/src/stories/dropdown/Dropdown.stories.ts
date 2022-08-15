import { Story, Meta } from '@storybook/angular/types-6-0';
import { componentWrapperDecorator } from '@storybook/angular';
import { DropdownComponent } from '../../app/components/dropdown/dropdown.component';

export default {
	title: 'Example/Dropdown',
	component: DropdownComponent,
	decorators: [componentWrapperDecorator(story => `<div class="p-5">${story}</div>`)],
	argTypes: {
		position: {
			options: ['right', 'left'],
			control: { type: 'select' },
			defaultValue: 'right'
		},
		size: {
			options: ['sm', 'md', 'lg', 'xl'],
			control: { type: 'select' },
			defaultValue: 'md'
		},
		buttonColor: {
			options: ['primary', 'success', 'warning', 'danger', 'grey'],
			control: { type: 'select' },
			defaultValue: 'primary'
		},
		buttonSize: {
			options: ['xs', 'sm', 'md', 'lg'],
			control: { type: 'select' },
			defaultValue: 'md'
		},
		buttonType: {
			options: ['default', 'outline', 'clear', 'text', 'link', 'icon'],
			control: { type: 'select' },
			defaultValue: 'default'
		},
		buttonTexture: {
			options: ['deep', 'light'],
			control: { type: 'select' },
			defaultValue: 'deep'
		},
		buttonText: {
			control: { type: 'text' }
		},
        class: {
			control: { type: 'text' }
		}
	},
	parameters: {
		actions: {
			handles: ['click']
		}
	}
} as Meta;

const Template: Story<DropdownComponent> = (args: DropdownComponent) => ({
	props: args,
	template: `<convoy-dropdown [buttonText]="buttonText" [position]="position" [size]="size" [buttonColor]="buttonColor" [buttonSize]="buttonSize" [buttonType]="buttonType" [buttonTexture]="buttonTexture">
                {{ngContent}}
               </convoy-dropdown>
              `
});

export const Base = Template.bind({});
Base.args = {
	ngContent: 'Dropdown content',
	position: 'right',
	size: 'md',
    class: 'p-3',
	buttonText: 'Dropdown toggle',
	buttonColor: 'primary',
	buttonSize: 'md',
	buttonType: 'default',
	buttonTexture: 'deep'
} as Partial<DropdownComponent>;
