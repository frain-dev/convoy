import { Story, Meta } from '@storybook/angular/types-6-0';
import { DropdownComponent } from '../../app/components/dropdown/dropdown.component';
import * as ButtonStories from '../button/Button.stories';

export default {
	title: 'Example/Dropdown',
	component: DropdownComponent,
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
		buttonFill: {
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
	template: ` <div class="flex justify-center h-300px">
                    <convoy-dropdown [buttonText]="buttonText" [position]="position" [size]="size" [buttonColor]="buttonColor" [buttonSize]="buttonSize" [buttonFill]="buttonFill" [buttonTexture]="buttonTexture">
                        {{ngContent}}
                    </convoy-dropdown>
                </div>
              `
});

export const Base = Template.bind({});
Base.args = {
	ngContent: 'Dropdown content',
	position: 'right',
	size: 'md',
	class: 'p-3',
	buttonText: ButtonStories.Base.args?.buttonText,
	buttonColor: ButtonStories.Base.args?.color,
	buttonSize: ButtonStories.Base.args?.size,
	buttonFill: ButtonStories.Base.args?.type,
	buttonTexture: ButtonStories.Base.args?.texture
} as Partial<DropdownComponent>;
