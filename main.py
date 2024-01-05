from flask import Flask, render_template, request, redirect, url_for, flash
from pymongo import MongoClient
from bson import ObjectId
# from flask_login import LoginManager, UserMixin

app = Flask(__name__)
client = MongoClient('mongodb+srv://admin:dblmrqNNPwmvmj8A@cluster0.zgjeaze.mongodb.net/?retryWrites=true&w=majority')
db = client['user_database']  # Create or connect to the 'user_database' MongoDB database
users_collection = db['users']
question_collection = db['questions']
questions = ['who are you?']
app.secret_key = 'tsetingteheserviceisnotgood'
# login_manager = LoginManager(app)


@app.route('/login')
def login():
    return render_template('login.html')


@app.route('/login', methods=['POST'])
def login_submit():
    # Retrieve username and password from the form
    username = request.form.get('username')
    password = request.form.get('password')

    # Validate the login credentials (dummy validation for illustration)
    user = users_collection.find_one({'username': username, 'password': password})

    if user:
        return redirect(url_for('user_dashboard'))
    else:
        return 'Invalid credentials'


# def user_authenticated():
#     # Check if the current user is authenticated
#     return current_user.is_authenticated

@app.route('/register')
def register():
    return render_template('register.html')


@app.route('/register', methods=['POST'])
def register_submit():
    # Retrieve registration details from the form
    first_name = request.form.get('first_name')
    last_name = request.form.get('last_name')
    email = request.form.get('email')
    phone = request.form.get('phone')
    username = request.form.get('username')
    password = request.form.get('password')

    user_data = {
        'first_name': first_name,
        'last_name': last_name,
        'email': email,
        'phone': phone,
        'username': username,
        'password': password
    }
    users_collection.insert_one(user_data)

    # Process the registration data (dummy processing for illustration)
    # return f'Registration successful for {username}'
    if request.method == 'POST':
        # Process registration logic (e.g., save user details to a database)

        # For demonstration purposes, let's assume the registration is successful
        registration_successful = True

        if registration_successful:
            flash('Registration successful! Please log in.', 'success')
            return redirect(url_for('login', registration_successful=True))

    return render_template('register.html', registration_successful='Registration successful for {username}')


@app.route('/admin_login')
def admin_login():
    return render_template('admin_login.html')

@app.route('/admin_login', methods=['POST'])
def admin_login_submit():
    username = request.form.get('username')
    password = request.form.get('password')
    token = request.form.get('token')

    ad = users_collection.find_one({'username': username, 'password': password, 'token': token})

    # Check if the admin credentials are valid (dummy validation for illustration)
    if ad:
        return redirect(url_for('admin_dashboard'))
    else:
        return 'Invalid admin credentials'

# @app.route('/admin_dashboard')
# def admin_dashboard():
#     with open('questions.txt', 'r') as file:
#         lines = file.readlines()
#     tools_and_questions = []
#
#     tools_and_questions = []
#
#     i = 0
#     while i < len(lines):
#         if i + 1 < len(lines) and lines[i].startswith('Tool:') and lines[i + 1].startswith('Question:'):
#             tool = lines[i].strip().replace('Tool: ', '')
#             question_lines = []
#
#             # Check for additional lines in the question
#             i += 1
#             while i < len(lines) and not lines[i].startswith('Tool:'):
#                 question_lines.append(lines[i].strip().replace('Question: ', ''))
#                 i += 1
#
#             question = '\n'.join(question_lines)
#             tools_and_questions.append({'Tool': tool, 'Question': question})
#         else:
#             i += 1
#
#     return render_template('admin_dashboard.html', tools_and_questions=tools_and_questions)

@app.route('/admin_dashboard')
def admin_dashboard():
    # Retrieve questions from MongoDB
    # if not user_authenticated():
    #     # If not authenticated, redirect to the login page
    #     return redirect(url_for('admin_login'))

    questions = question_collection.find({}, {'tool': 1, 'question_text': 1})
    tools_and_questions = [
        {'_id': str(question.get('_id', '')), 'tool': question.get('tool', ''),
         'question_text': question.get('question_text', '')}
        for question in questions
    ]

    # Structure the data for rendering in the template
    # tools_and_questions = [{'Tool': question['tool'], 'Question': question['question_text']} for question in questions]
    # quests = get_questions_from_mongodb()

    return render_template('admin_dashboard.html', tools_and_questions=tools_and_questions)

# @app.route('/create_question', methods=['GET', 'POST'])
# def create_question():
#         if request.method == 'POST':
#             tool = request.form.get('tool')
#             question = request.form.get('question')
#             question = request.form.get('question').replace('\n', ' ')
#
#             # Save the tool and question to a text file
#             with open('questions.txt', 'a') as file:
#                 file.write(f'\nTool: {tool}\nQuestion: {question}\n')
#
#             return redirect(url_for('admin_dashboard'))
#         return render_template('create_question.html')

@app.route('/create_question', methods=['GET', 'POST'])
def create_question():
    if request.method == 'POST':
        tool = request.form.get('tool')
        question_text = request.form.get('question')

        # Create a document to insert into the MongoDB collection
        question_document = {
            'tool': tool,
            'question_text': question_text
        }

        # Insert the document into the collection
        question_collection.insert_one(question_document)

        return redirect(url_for('admin_dashboard'))

    return render_template('create_question.html')

# @app.route('/user_dashboard')
# def user_dashboard():
#      with open('questions.txt', 'r') as file:
#           lines = file.readlines()
#
#      tools_and_questions = [{'Tool': lines[i].strip().replace('Tool: ',''), 'Question': lines[i + 1].strip().replace('Question: ', '')} for i in range(0, len(lines), 3)]
#
#      return render_template("user_dashboard.html", tools_and_questions=tools_and_questions)

@app.route('/user_dashboard')
def user_dashboard():
    # Retrieve questions and tools from MongoDB
    questions = question_collection.find({}, {'_id': 0, 'tool': 1, 'question_text': 1})

    # Structure the data for rendering in the template
    tools_and_questions = [{'Tool': question['tool'], 'Question': question['question_text']} for question in questions]

    return render_template('user_dashboard.html', tools_and_questions=tools_and_questions)

# def update_question_in_file(tool, question, new_question_text):
#     # Read the content of the file
#     with open('questions.txt', 'r') as file:
#         lines = file.readlines()
#
#     # Update the question in the list
#     for i, line in enumerate(lines):
#         if f'Tool: {tool}' in line and f'Question: {question}' in lines[i + 1]:
#             lines[i + 1] = f'Question: {new_question_text}\n'
#             flash('Question updated successfully', 'success')
#
#     # Write the updated content back to the file
#     with open('questions.txt', 'w') as file:
#         file.writelines(lines)

# @app.route('/edit_question/<tool>/<question>', methods=['GET', 'POST'])
# def edit_question(tool, question):
#     if request.method == 'POST':
#         new_question_text = request.form.get('new_question_text')
#         update_question_in_file(tool, question, new_question_text)
#         return redirect(url_for('admin_dashboard'))
#
#     return render_template('edit_question.html', tool=tool, question=question)

@app.route('/edit_question/<question_id>', methods=['GET', 'POST'])
def edit_question(question_id):
    if request.method == 'POST':
        new_question_text = request.form.get('new_question_text')

        # Update the question in MongoDB
        question_collection.update_one({'_id': ObjectId(question_id)}, {'$set': {'question_text': new_question_text}})

        return redirect(url_for('admin_dashboard'))

    # Retrieve the existing question for editing
    question = question_collection.find_one({'_id': ObjectId(question_id)}, {'_id': 0, 'tool': 1, 'question_text': 1})
    existing_question = {'Tool': question['tool'], 'Question': question['question_text']}

    return render_template('edit_question.html', question_id=question_id, existing_question=existing_question)


@app.route('/logout', methods=['POST'])
def logout():
    # Perform any logout actions if needed
    # ...

    # Redirect to the home page
    return redirect(url_for('home'))

@app.route('/')
def home():
    return render_template('default.html')

if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0', port=5000)
